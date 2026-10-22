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

package transformer_test

import (
	"errors"
	"testing"

	transformertypes "github.com/konveyor/move2kube/types/transformer"
)

type sampleConfig struct {
	Name  string   `yaml:"name"`
	Ports []int    `yaml:"ports"`
	Tags  []string `yaml:"tags"`
}

func TestArtifactGetConfig(t *testing.T) {
	a := transformertypes.Artifact{
		Name: "svc",
		Configs: map[transformertypes.ConfigType]interface{}{
			"fromMap":    map[string]interface{}{"name": "web", "ports": []interface{}{80, 443}, "tags": []string{"a"}},
			"fromStruct": sampleConfig{Name: "direct"},
			"bad":        map[string]interface{}{"ports": "not-a-list"},
		},
	}

	t.Run("decodes a generic map", func(t *testing.T) {
		got := sampleConfig{}
		if err := a.GetConfig("fromMap", &got); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != "web" || len(got.Ports) != 2 || got.Ports[1] != 443 || got.Tags[0] != "a" {
			t.Fatalf("unexpected config: %+v", got)
		}
	})

	t.Run("decodes a typed struct", func(t *testing.T) {
		got := sampleConfig{}
		if err := a.GetConfig("fromStruct", &got); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != "direct" {
			t.Fatalf("unexpected config: %+v", got)
		}
	})

	t.Run("missing config", func(t *testing.T) {
		if err := a.GetConfig("nope", &sampleConfig{}); err == nil {
			t.Fatal("expected an error for a missing config")
		}
	})

	t.Run("undecodable config", func(t *testing.T) {
		if err := a.GetConfig("bad", &sampleConfig{}); err == nil {
			t.Fatal("expected a decode error")
		}
	})
}

func TestTransformerDisabledError(t *testing.T) {
	err := &transformertypes.TransformerDisabledError{Err: errors.New("no java")}
	if got, want := err.Error(), "Transformer Disabled : no java"; got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}
