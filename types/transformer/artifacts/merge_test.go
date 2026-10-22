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

package artifacts_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/konveyor/move2kube/types/transformer/artifacts"
)

func TestContainerizationOptionsConfigMerge(t *testing.T) {
	co := artifacts.ContainerizationOptionsConfig{"a", "b"}
	if !co.Merge(artifacts.ContainerizationOptionsConfig{"b", "c"}) {
		t.Fatal("merge by value should succeed")
	}
	if !co.Merge(&artifacts.ContainerizationOptionsConfig{"d"}) {
		t.Fatal("merge by pointer should succeed")
	}
	if diff := cmp.Diff(artifacts.ContainerizationOptionsConfig{"a", "b", "c", "d"}, co); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
	if co.Merge("wrong type") {
		t.Fatal("merge with the wrong type should fail")
	}
}

func TestNewImagesMerge(t *testing.T) {
	ni := artifacts.NewImages{ImageNames: []string{"x"}}
	if !ni.Merge(artifacts.NewImages{ImageNames: []string{"x", "y"}}) || !ni.Merge(&artifacts.NewImages{ImageNames: []string{"z"}}) {
		t.Fatal("merge should succeed for value and pointer")
	}
	if diff := cmp.Diff([]string{"x", "y", "z"}, ni.ImageNames); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
	if ni.Merge(42) {
		t.Fatal("merge with the wrong type should fail")
	}
}

func TestMavenConfigMerge(t *testing.T) {
	base := func() artifacts.MavenConfig {
		return artifacts.MavenConfig{MavenAppName: "app", PackagingType: artifacts.WarPackaging, MavenProfiles: []string{"dev"}}
	}

	mc := base()
	if !mc.Merge(&artifacts.MavenConfig{MavenAppName: "app", PackagingType: artifacts.WarPackaging, MavenProfiles: []string{"dev", "prod"}}) {
		t.Fatal("compatible configs should merge")
	}
	if diff := cmp.Diff([]string{"dev", "prod"}, mc.MavenProfiles); diff != "" {
		t.Fatalf("profiles mismatch (-want +got):\n%s", diff)
	}

	for name, other := range map[string]artifacts.MavenConfig{
		"different app name":  {MavenAppName: "other", PackagingType: artifacts.WarPackaging},
		"different packaging": {MavenAppName: "app", PackagingType: artifacts.JarPackaging},
	} {
		mc := base()
		if mc.Merge(other) {
			t.Errorf("%s: merge should be refused", name)
		}
	}
	mc = base()
	if mc.Merge(artifacts.GradleConfig{}) {
		t.Fatal("merge with the wrong type should fail")
	}
}

func TestGradleConfigMerge(t *testing.T) {
	gc := artifacts.GradleConfig{PackagingType: artifacts.JarPackaging}
	if !gc.Merge(artifacts.GradleConfig{PackagingType: artifacts.JarPackaging}) {
		t.Fatal("same packaging should merge")
	}
	if gc.Merge(&artifacts.GradleConfig{PackagingType: artifacts.EarPackaging}) {
		t.Fatal("different packaging should not merge")
	}
	if gc.Merge(nil) {
		t.Fatal("merge with nil should fail")
	}
}

func TestSpringBootConfigMerge(t *testing.T) {
	t.Run("merges profiles, ports and fills in the version", func(t *testing.T) {
		sb := artifacts.SpringBootConfig{
			SpringBootAppName:      "app",
			SpringBootProfiles:     &[]string{"dev"},
			SpringBootProfilePorts: map[string][]int32{"dev": {8080}},
		}
		ok := sb.Merge(artifacts.SpringBootConfig{
			SpringBootAppName:      "app",
			SpringBootVersion:      "2.7.0",
			SpringBootProfiles:     &[]string{"dev", "prod"},
			SpringBootProfilePorts: map[string][]int32{"dev": {8080, 8081}, "prod": {80}},
		})
		if !ok {
			t.Fatal("compatible configs should merge")
		}
		want := artifacts.SpringBootConfig{
			SpringBootAppName:      "app",
			SpringBootVersion:      "2.7.0",
			SpringBootProfiles:     &[]string{"dev", "prod"},
			SpringBootProfilePorts: map[string][]int32{"dev": {8080, 8081}, "prod": {80}},
		}
		if diff := cmp.Diff(want, sb); diff != "" {
			t.Fatalf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("nil port map is initialised", func(t *testing.T) {
		sb := artifacts.SpringBootConfig{SpringBootAppName: "app", SpringBootProfiles: &[]string{}}
		if !sb.Merge(&artifacts.SpringBootConfig{SpringBootAppName: "app", SpringBootProfiles: &[]string{}, SpringBootProfilePorts: map[string][]int32{"p": {1}}}) {
			t.Fatal("merge should succeed")
		}
		if diff := cmp.Diff(map[string][]int32{"p": {1}}, sb.SpringBootProfilePorts); diff != "" {
			t.Fatalf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("different app names are refused", func(t *testing.T) {
		sb := artifacts.SpringBootConfig{SpringBootAppName: "a"}
		if sb.Merge(artifacts.SpringBootConfig{SpringBootAppName: "b"}) {
			t.Fatal("merge should be refused")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		sb := artifacts.SpringBootConfig{}
		if sb.Merge("x") {
			t.Fatal("merge with the wrong type should fail")
		}
	})
}

func TestConfigTypesRegistry(t *testing.T) {
	for _, name := range []string{"IR", "NewImages", "MavenConfig", "GradleConfig", "SpringBootConfig", "ContainerizationOptionsConfig", "ClusterMetadata"} {
		typ, ok := artifacts.ConfigTypes[name]
		if !ok {
			t.Errorf("config type %q is not registered: %v", name, reflect.ValueOf(artifacts.ConfigTypes).MapKeys())
			continue
		}
		if typ.Name() != name {
			t.Errorf("config type %q maps to %v", name, typ)
		}
	}
}
