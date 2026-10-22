/*
 *  Copyright IBM Corporation 2021
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package apiresource

import (
	"fmt"
	"sort"
	"strings"

	"github.com/konveyor/move2kube/common"
	"github.com/konveyor/move2kube/qaengine"
	"github.com/konveyor/move2kube/types"
	collecttypes "github.com/konveyor/move2kube/types/collection"
	irtypes "github.com/konveyor/move2kube/types/ir"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	networking "k8s.io/kubernetes/pkg/apis/networking"
)

const (
	networkPolicyKind = "NetworkPolicy"
	networkSelector   = types.GroupName + "/network"

	// KubernetesNetworkPolicyType generates networking.k8s.io/v1 NetworkPolicies
	KubernetesNetworkPolicyType = "Kubernetes"
	// CalicoNetworkPolicyType generates projectcalico.org/v3 NetworkPolicies
	CalicoNetworkPolicyType = "Calico"
)

// NetworkPolicy handles NetworkPolicy objects
type NetworkPolicy struct {
}

// getSupportedKinds returns all kinds supported by the class
func (d *NetworkPolicy) getSupportedKinds() []string {
	return []string{networkPolicyKind}
}

// createNewResources converts ir to runtime objects
func (d *NetworkPolicy) createNewResources(ir irtypes.EnhancedIR, supportedKinds []string, targetCluster collecttypes.ClusterMetadata) []runtime.Object {
	objs := []runtime.Object{}
	if !common.IsPresent(supportedKinds, networkPolicyKind) {
		logrus.Errorf("Could not find a valid resource type in cluster to create a NetworkPolicy")
		return nil
	}

	networks := []string{}
	for _, service := range ir.Services {
		networks = common.AppendIfNotPresent(networks, service.Networks...)
	}
	if len(networks) == 0 {
		return objs
	}
	sort.Strings(networks)
	policyType := getNetworkPolicyType()
	for _, net := range networks {
		logrus.Debugf("Network %s is detected at Source, shall be converted to equivalent %s NetworkPolicy at Destination", net, policyType)
		var obj runtime.Object
		var err error
		if policyType == CalicoNetworkPolicyType {
			obj, err = d.createCalicoNetworkPolicy(net)
		} else {
			obj, err = d.createNetworkPolicy(net)
		}
		if err != nil {
			logrus.Warnf("Unable to create Network Policy for network %v : %s", net, err)
			continue
		}
		objs = append(objs, obj)
	}
	return objs
}

// getNetworkPolicyType asks which kind of network policy should be generated
func getNetworkPolicyType() string {
	options := []string{KubernetesNetworkPolicyType, CalicoNetworkPolicyType}
	policyType := qaengine.FetchSelectAnswer(
		common.ConfigTransformersKubernetesNetworkPolicyTypeKey,
		"Which type of NetworkPolicy should be generated for the networks of the services?",
		[]string{"Calico NetworkPolicies (projectcalico.org/v3) require Calico and its API server or calicoctl on the target cluster."},
		KubernetesNetworkPolicyType,
		options,
		nil,
	)
	if !common.IsPresent(options, policyType) {
		logrus.Warnf("Unsupported NetworkPolicy type '%s'. Valid types are %v. Defaulting to %s", policyType, options, KubernetesNetworkPolicyType)
		return KubernetesNetworkPolicyType
	}
	return policyType
}

// convertToClusterSupportedKinds converts kinds to cluster supported kinds
func (d *NetworkPolicy) convertToClusterSupportedKinds(obj runtime.Object, supportedKinds []string, otherobjs []runtime.Object, _ irtypes.EnhancedIR, targetCluster collecttypes.ClusterMetadata) ([]runtime.Object, bool) {
	if common.IsPresent(d.getSupportedKinds(), obj.GetObjectKind().GroupVersionKind().Kind) {
		return []runtime.Object{obj}, true
	}
	return nil, false
}

// CreateNetworkPolicy initializes Network policy
func (d *NetworkPolicy) createNetworkPolicy(networkName string) (*networking.NetworkPolicy, error) {

	np := &networking.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkPolicyKind,
			APIVersion: networking.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: networkName,
		},
		Spec: networking.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{networkSelector + "-" + networkName: common.AnnotationLabelValue},
			},
			Ingress: []networking.NetworkPolicyIngressRule{{
				From: []networking.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: getNetworkPolicyLabels([]string{networkName}),
					},
				}},
			}},
		},
	}

	return np, nil
}

// createCalicoNetworkPolicy creates a Calico network policy equivalent to the one created by createNetworkPolicy:
// pods on the network only accept ingress traffic from other pods on the same network.
func (d *NetworkPolicy) createCalicoNetworkPolicy(networkName string) (*CalicoNetworkPolicy, error) {
	networkPodSelector := getCalicoSelector(getNetworkPolicyLabels([]string{networkName}))
	np := &CalicoNetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkPolicyKind,
			APIVersion: CalicoSchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: networkName,
		},
		Spec: CalicoNetworkPolicySpec{
			Selector: networkPodSelector,
			Types:    []string{calicoIngressPolicyType},
			Ingress: []CalicoRule{{
				Action: calicoAllowAction,
				Source: CalicoEntityRule{Selector: networkPodSelector},
			}},
		},
	}
	return np, nil
}

// getCalicoSelector converts labels into a Calico selector expression that matches all of them
func getCalicoSelector(labels map[string]string) string {
	keys := []string{}
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	exprs := []string{}
	for _, key := range keys {
		exprs = append(exprs, fmt.Sprintf("%s == '%s'", key, labels[key]))
	}
	return strings.Join(exprs, " && ")
}

func getNetworkPolicyLabels(networks []string) map[string]string {
	networklabels := map[string]string{}
	for _, network := range networks {
		networklabels[networkSelector+"-"+network] = common.AnnotationLabelValue
	}
	return networklabels
}
