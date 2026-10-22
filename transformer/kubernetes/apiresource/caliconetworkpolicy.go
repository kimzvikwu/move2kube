/*
 *  Copyright IBM Corporation 2026
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// The Calico types below cover only the fields move2kube generates. They are defined here
// rather than imported from github.com/projectcalico/api to avoid pulling in that module.
// See https://docs.tigera.io/calico/latest/reference/resources/networkpolicy

const (
	calicoAllowAction       = "Allow"
	calicoIngressPolicyType = "Ingress"
)

var (
	// CalicoSchemeGroupVersion is the group version of the Calico v3 API
	CalicoSchemeGroupVersion = schema.GroupVersion{Group: "projectcalico.org", Version: "v3"}
)

// CalicoNetworkPolicy is a namespaced Calico (projectcalico.org/v3) NetworkPolicy
type CalicoNetworkPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              CalicoNetworkPolicySpec `json:"spec,omitempty"`
}

// CalicoNetworkPolicySpec is the spec of a Calico NetworkPolicy
type CalicoNetworkPolicySpec struct {
	// Selector selects the endpoints the policy applies to, using the Calico selector syntax
	Selector string       `json:"selector,omitempty"`
	Types    []string     `json:"types,omitempty"`
	Ingress  []CalicoRule `json:"ingress,omitempty"`
}

// CalicoRule is a Calico policy rule
type CalicoRule struct {
	Action string           `json:"action"`
	Source CalicoEntityRule `json:"source,omitempty"`
}

// CalicoEntityRule matches the source or destination of a Calico policy rule
type CalicoEntityRule struct {
	Selector string `json:"selector,omitempty"`
}

// DeepCopyObject implements runtime.Object
func (in *CalicoNetworkPolicy) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := &CalicoNetworkPolicy{TypeMeta: in.TypeMeta, Spec: in.Spec}
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec.Types = append([]string(nil), in.Spec.Types...)
	out.Spec.Ingress = append([]CalicoRule(nil), in.Spec.Ingress...)
	return out
}
