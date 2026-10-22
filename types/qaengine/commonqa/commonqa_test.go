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

package commonqa

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/konveyor/move2kube/common"
	"github.com/konveyor/move2kube/qaengine"
	"github.com/konveyor/move2kube/types/ir"
	qatypes "github.com/konveyor/move2kube/types/qaengine"
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

var answers = &answerEngine{answers: map[string]interface{}{}}

func TestMain(m *testing.M) {
	common.IgnoreEnvironment = true // don't read the developer's docker config
	qaengine.AddEngine(qaengine.NewDefaultEngine())
	if err := qaengine.AddEngineHighestPriority(answers); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// answer sets the answers for a single test and clears them afterwards.
func answer(t *testing.T, a map[string]interface{}) {
	t.Helper()
	answers.answers = a
	t.Cleanup(func() { answers.answers = map[string]interface{}{} })
}

func TestImageRegistry(t *testing.T) {
	if got := ImageRegistry(); got != "quay.io" {
		t.Fatalf("want the default registry quay.io, got %q", got)
	}
	answer(t, map[string]interface{}{common.ConfigImageRegistryURLKey: "registry.example.com"})
	if got := ImageRegistry(); got != "registry.example.com" {
		t.Fatalf("want the configured registry, got %q", got)
	}
}

func TestImageRegistryNamespace(t *testing.T) {
	if got := ImageRegistryNamespace(); got != common.ProjectName {
		t.Fatalf("want the project name %q, got %q", common.ProjectName, got)
	}
}

func TestIngressHost(t *testing.T) {
	if got := IngressHost("myapp.example.com", "default"); got != "myapp.example.com" {
		t.Fatalf("want the default host, got %q", got)
	}
	key := common.JoinQASubKeys(common.ConfigTargetKey, `"prod"`, common.ConfigIngressHostKeySuffix)
	answer(t, map[string]interface{}{key: "prod.example.com"})
	if got := IngressHost("myapp.example.com", "prod"); got != "prod.example.com" {
		t.Fatalf("want the answer for the prod cluster, got %q", got)
	}
}

func TestMinimumReplicaCount(t *testing.T) {
	if got := MinimumReplicaCount("2"); got != "2" {
		t.Fatalf("want the default 2, got %q", got)
	}
	answer(t, map[string]interface{}{common.ConfigMinReplicasKey: "5"})
	if got := MinimumReplicaCount("2"); got != "5" {
		t.Fatalf("want 5, got %q", got)
	}
	// An invalid answer is rejected by the validator, so the default engine
	// supplies the default instead.
	answer(t, map[string]interface{}{common.ConfigMinReplicasKey: "-1"})
	if got := MinimumReplicaCount("2"); got != "2" {
		t.Fatalf("a negative replica count should be rejected, got %q", got)
	}
}

func TestGetPortsForService(t *testing.T) {
	if got := GetPortsForService(nil, "svc"); len(got) != 0 {
		t.Fatalf("no detected ports should mean no question and no ports, got %v", got)
	}
	if diff := cmp.Diff([]int32{8080, 9090}, GetPortsForService([]int32{8080, 9090}, "svc")); diff != "" {
		t.Fatalf("all detected ports should be selected by default (-want +got):\n%s", diff)
	}
	key := common.JoinQASubKeys(common.ConfigServicesKey, "svc", common.ConfigPortsForServiceKeySegment)
	answer(t, map[string]interface{}{key: []string{"9090"}})
	if diff := cmp.Diff([]int32{9090}, GetPortsForService([]int32{8080, 9090}, "svc")); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestGetPortForService(t *testing.T) {
	if got := GetPortForService([]int32{3000, 4000}, "svc"); got != 3000 {
		t.Fatalf("want the first detected port, got %d", got)
	}
	if got := GetPortForService(nil, "svc"); got != common.DefaultServicePort {
		t.Fatalf("want the default service port, got %d", got)
	}

	key := common.JoinQASubKeys(common.ConfigServicesKey, "svc", common.ConfigPortForServiceKeySegment)
	tests := map[string]int32{
		"4000":       4000,
		" 4000 ":     4000,
		"not-a-port": common.DefaultServicePort,
		"70000":      common.DefaultServicePort,
	}
	for ans, want := range tests {
		t.Run(ans, func(t *testing.T) {
			answer(t, map[string]interface{}{key: ans})
			if got := GetPortForService([]int32{3000}, "svc"); got != want {
				t.Fatalf("answer %q: want %d, got %d", ans, want, got)
			}
		})
	}
}

func TestGetDeploymentType(t *testing.T) {
	if got := GetDeploymentType("svc"); got != ir.DeploymentTypeDeployment {
		t.Fatalf("want Deployment by default, got %q", got)
	}
	key := common.JoinQASubKeys(common.ConfigServicesKey, "svc", common.ConfigDeploymentTypeKey)
	for _, want := range []ir.DeploymentType{ir.DeploymentTypeStatefulSet, ir.DeploymentTypeArgoRollout, ir.DeploymentTypeDeployment} {
		t.Run(string(want), func(t *testing.T) {
			answer(t, map[string]interface{}{key: string(want)})
			if got := GetDeploymentType("svc"); got != want {
				t.Fatalf("want %q, got %q", want, got)
			}
		})
	}
}
