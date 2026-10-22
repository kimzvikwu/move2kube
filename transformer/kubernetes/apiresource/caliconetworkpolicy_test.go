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
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/konveyor/move2kube/common"
	"github.com/konveyor/move2kube/qaengine"
	collecttypes "github.com/konveyor/move2kube/types/collection"
	irtypes "github.com/konveyor/move2kube/types/ir"
	qatypes "github.com/konveyor/move2kube/types/qaengine"
	"gopkg.in/yaml.v3"
	"k8s.io/kubernetes/pkg/apis/networking"
)

// answerEngine answers the problems listed in answers and leaves the rest to
// the default engine that sits below it.
type answerEngine struct {
	answers map[string]interface{}
}

func (*answerEngine) StartEngine() error        { return nil }
func (*answerEngine) IsInteractiveEngine() bool { return false }
func (e *answerEngine) FetchAnswer(prob qatypes.Problem) (qatypes.Problem, error) {
	if ans, ok := e.answers[prob.ID]; ok {
		if err := prob.SetAnswer(ans, true); err != nil {
			return prob, err
		}
	}
	return prob, nil
}

var testAnswers = &answerEngine{answers: map[string]interface{}{}}

func TestMain(m *testing.M) {
	qaengine.AddEngine(qaengine.NewDefaultEngine())
	if err := qaengine.AddEngineHighestPriority(testAnswers); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func useNetworkPolicyType(t *testing.T, policyType string) {
	t.Helper()
	testAnswers.answers = map[string]interface{}{common.ConfigTransformersKubernetesNetworkPolicyTypeKey: policyType}
	t.Cleanup(func() { testAnswers.answers = map[string]interface{}{} })
}

func irWithNetworks(serviceNetworks map[string][]string) irtypes.EnhancedIR {
	ir := irtypes.NewEnhancedIRFromIR(irtypes.NewIR())
	ir.Services = map[string]irtypes.Service{}
	for name, networks := range serviceNetworks {
		svc := irtypes.NewServiceWithName(name)
		svc.Networks = networks
		ir.Services[name] = svc
	}
	return ir
}

func wantCalicoNetworkPolicy(network string) *CalicoNetworkPolicy {
	sel := "move2kube.konveyor.io/network-" + network + " == 'true'"
	np := &CalicoNetworkPolicy{
		Spec: CalicoNetworkPolicySpec{
			Selector: sel,
			Types:    []string{"Ingress"},
			Ingress:  []CalicoRule{{Action: "Allow", Source: CalicoEntityRule{Selector: sel}}},
		},
	}
	np.Kind = "NetworkPolicy"
	np.APIVersion = "projectcalico.org/v3"
	np.Name = network
	return np
}

func TestGetCalicoSelector(t *testing.T) {
	tests := []struct {
		labels map[string]string
		want   string
	}{
		{map[string]string{}, ""},
		{map[string]string{"a": "true"}, "a == 'true'"},
		{map[string]string{"b": "2", "a": "1"}, "a == '1' && b == '2'"},
	}
	for _, tt := range tests {
		if got := getCalicoSelector(tt.labels); got != tt.want {
			t.Errorf("getCalicoSelector(%v) = %q, want %q", tt.labels, got, tt.want)
		}
	}
}

func TestCreateNewResourcesNetworkPolicyType(t *testing.T) {
	netPolicy := NetworkPolicy{}
	supKinds := []string{networkPolicyKind}

	t.Run("Kubernetes is the default", func(t *testing.T) {
		objs := netPolicy.createNewResources(irWithNetworks(map[string][]string{"svc1": {"net1"}}), supKinds, collecttypes.ClusterMetadata{})
		if len(objs) != 1 {
			t.Fatalf("want 1 object, got %d", len(objs))
		}
		if _, ok := objs[0].(*networking.NetworkPolicy); !ok {
			t.Fatalf("want a Kubernetes NetworkPolicy by default, got %T", objs[0])
		}
	})

	t.Run("Calico", func(t *testing.T) {
		useNetworkPolicyType(t, CalicoNetworkPolicyType)
		ir := irWithNetworks(map[string][]string{"svc1": {"net1", "shared"}, "svc2": {"net2", "shared"}})
		objs := netPolicy.createNewResources(ir, supKinds, collecttypes.ClusterMetadata{})
		got := []*CalicoNetworkPolicy{}
		for _, obj := range objs {
			np, ok := obj.(*CalicoNetworkPolicy)
			if !ok {
				t.Fatalf("want only Calico NetworkPolicies, got %T", obj)
			}
			got = append(got, np)
		}
		sort.Slice(got, func(i, j int) bool { return got[i].Name < got[j].Name })
		want := []*CalicoNetworkPolicy{wantCalicoNetworkPolicy("net1"), wantCalicoNetworkPolicy("net2"), wantCalicoNetworkPolicy("shared")}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("one policy per distinct network expected (-want +got):\n%s", diff)
		}
	})

	t.Run("no networks means no question and no policies", func(t *testing.T) {
		useNetworkPolicyType(t, "not-a-valid-option") // would be rejected if the question were asked
		objs := netPolicy.createNewResources(irWithNetworks(map[string][]string{"svc1": nil}), supKinds, collecttypes.ClusterMetadata{})
		if len(objs) != 0 {
			t.Fatalf("want no objects, got %v", objs)
		}
	})

	t.Run("unsupported cluster", func(t *testing.T) {
		useNetworkPolicyType(t, CalicoNetworkPolicyType)
		if objs := netPolicy.createNewResources(irWithNetworks(map[string][]string{"svc1": {"net1"}}), nil, collecttypes.ClusterMetadata{}); objs != nil {
			t.Fatalf("want nil when the cluster doesn't support NetworkPolicy, got %v", objs)
		}
	})
}

func TestCalicoNetworkPolicyDeepCopy(t *testing.T) {
	orig := wantCalicoNetworkPolicy("net1")
	orig.Labels = map[string]string{"a": "b"}
	cp := orig.DeepCopyObject().(*CalicoNetworkPolicy)
	if diff := cmp.Diff(orig, cp); diff != "" {
		t.Fatalf("copy differs (-orig +copy):\n%s", diff)
	}
	cp.Labels["a"] = "changed"
	cp.Spec.Types[0] = "Egress"
	cp.Spec.Ingress[0].Action = "Deny"
	if orig.Labels["a"] != "b" || orig.Spec.Types[0] != "Ingress" || orig.Spec.Ingress[0].Action != "Allow" {
		t.Fatalf("modifying the copy changed the original: %+v", orig)
	}
	var nilNP *CalicoNetworkPolicy
	if nilNP.DeepCopyObject() != nil {
		t.Fatal("deep copy of nil should be nil")
	}
}

func TestTransformIRAndPersistCalico(t *testing.T) {
	useNetworkPolicyType(t, CalicoNetworkPolicyType)
	cluster := collecttypes.NewClusterMetadata("test")
	cluster.Spec.APIKindVersionMap = map[string][]string{networkPolicyKind: {"networking.k8s.io/v1"}}
	outDir := t.TempDir()
	ir := irWithNetworks(map[string][]string{"svc1": {"net1"}, "svc2": {"net1"}})

	files, err := TransformIRAndPersist(ir, outDir, []IAPIResource{new(NetworkPolicy)}, cluster, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "net1-networkpolicy.yaml" {
		t.Fatalf("want a single net1-networkpolicy.yaml, got %v", files)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid yaml: %v\n%s", err, data)
	}
	sel := "move2kube.konveyor.io/network-net1 == 'true'"
	want := map[string]interface{}{
		"apiVersion": "projectcalico.org/v3",
		"kind":       "NetworkPolicy",
		"metadata":   map[string]interface{}{"name": "net1", "creationTimestamp": nil},
		"spec": map[string]interface{}{
			"selector": sel,
			"types":    []interface{}{"Ingress"},
			"ingress":  []interface{}{map[string]interface{}{"action": "Allow", "source": map[string]interface{}{"selector": sel}}},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("written yaml mismatch (-want +got):\n%s\n%s", diff, data)
	}
}
