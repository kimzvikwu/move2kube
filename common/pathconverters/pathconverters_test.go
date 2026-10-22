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

package pathconverters

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/konveyor/move2kube/common"
)

type inner struct {
	Path    string `m2kpath:"normal"`
	NotPath string
}

type tagged struct {
	Normal      string `m2kpath:"normal"`
	Untagged    string
	Slice       []string          `m2kpath:"normal"`
	Map         map[string]string `m2kpath:"normal"`
	KeyedMap    map[string]string `m2kpath:"keys:a,b"`
	Kind        string
	Conditional string      `m2kpath:"if:Kind:in:file,dir"`
	Ptr         *inner      `m2kpath:"normal"`
	NilPtr      *inner      `m2kpath:"normal"`
	Nested      inner       // untagged struct: tags on its own fields still apply
	Iface       interface{} `m2kpath:"normal"`
	Number      int         `m2kpath:"normal"`
}

func upper(s string) (string, error) { return strings.ToUpper(s), nil }

func TestProcessPaths(t *testing.T) {
	t.Run("converts only tagged fields", func(t *testing.T) {
		obj := tagged{
			Normal:      "n",
			Untagged:    "u",
			Slice:       []string{"s1", "s2"},
			Map:         map[string]string{"k": "v"},
			KeyedMap:    map[string]string{"a": "va", "b": "vb", "c": "vc"},
			Kind:        "file",
			Conditional: "cond",
			Ptr:         &inner{Path: "p", NotPath: "np"},
			Nested:      inner{Path: "nested", NotPath: "keep"},
			Iface:       &inner{Path: "i"},
			Number:      7,
		}
		if err := ProcessPaths(&obj, upper); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := tagged{
			Normal:      "N",
			Untagged:    "u",
			Slice:       []string{"S1", "S2"},
			Map:         map[string]string{"k": "V"},
			KeyedMap:    map[string]string{"a": "VA", "b": "VB", "c": "vc"},
			Kind:        "file",
			Conditional: "COND",
			Ptr:         &inner{Path: "P", NotPath: "np"},
			Nested:      inner{Path: "NESTED", NotPath: "keep"},
			Iface:       &inner{Path: "I"},
			Number:      7,
		}
		if diff := cmp.Diff(want, obj); diff != "" {
			t.Fatalf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("condition not met leaves field alone", func(t *testing.T) {
		obj := tagged{Kind: "url", Conditional: "cond"}
		if err := ProcessPaths(&obj, upper); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if obj.Conditional != "cond" {
			t.Fatalf("expected Conditional to be unchanged, got %q", obj.Conditional)
		}
	})

	t.Run("converter error is propagated", func(t *testing.T) {
		obj := tagged{Normal: "n"}
		err := ProcessPaths(&obj, func(string) (string, error) { return "", fmt.Errorf("boom") })
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("expected the converter error to be returned, got %v", err)
		}
	})

	t.Run("invalid tag is an error", func(t *testing.T) {
		type bad struct {
			P string `m2kpath:"bogus"`
		}
		obj := bad{P: "x"}
		if err := ProcessPaths(&obj, upper); err == nil {
			t.Fatal("expected an error for an invalid m2kpath tag")
		}
	})

	t.Run("keys tag on map with non-string keys is an error", func(t *testing.T) {
		type bad struct {
			M map[int]string `m2kpath:"keys:a"`
		}
		obj := bad{M: map[int]string{1: "x"}}
		if err := ProcessPaths(&obj, upper); err == nil {
			t.Fatal("expected an error for non-string map keys")
		}
	})
}

func TestMakePlanPathsAbsolute(t *testing.T) {
	sourcePath := filepath.Join(string(filepath.Separator), "src")
	assetsPath := filepath.Join(string(filepath.Separator), "assets")
	abs := filepath.Join(string(filepath.Separator), "already", "abs")
	type paths struct {
		Rel    string   `m2kpath:"normal"`
		Asset  string   `m2kpath:"normal"`
		Abs    string   `m2kpath:"normal"`
		Empty  string   `m2kpath:"normal"`
		Others []string `m2kpath:"normal"`
	}
	obj := paths{
		Rel:    filepath.Join("a", "b"),
		Asset:  filepath.Join(common.AssetsDir, "x.yaml"),
		Abs:    abs,
		Others: []string{"c"},
	}
	if err := MakePlanPathsAbsolute(&obj, sourcePath, assetsPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := paths{
		Rel:    filepath.Join(sourcePath, "a", "b"),
		Asset:  filepath.Join(assetsPath, common.AssetsDir, "x.yaml"),
		Abs:    abs,
		Others: []string{filepath.Join(sourcePath, "c")},
	}
	if diff := cmp.Diff(want, obj); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestChangePaths(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "old")
	dest := filepath.Join(root, "new")
	type paths struct {
		P string `m2kpath:"normal"`
	}

	t.Run("rebases a path under a mapped root", func(t *testing.T) {
		obj := paths{P: filepath.Join(src, "svc", "Dockerfile")}
		if err := ChangePaths(&obj, map[string]string{src: dest}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := filepath.Join(dest, "svc", "Dockerfile"); obj.P != want {
			t.Fatalf("want %q, got %q", want, obj.P)
		}
	})

	t.Run("empty path is left alone", func(t *testing.T) {
		obj := paths{}
		if err := ChangePaths(&obj, map[string]string{src: dest}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if obj.P != "" {
			t.Fatalf("expected empty path, got %q", obj.P)
		}
	})

	t.Run("relative path is an error", func(t *testing.T) {
		obj := paths{P: "relative/path"}
		if err := ChangePaths(&obj, map[string]string{src: dest}); err == nil {
			t.Fatal("expected an error for a relative path")
		}
	})

	t.Run("path outside every root is an error", func(t *testing.T) {
		obj := paths{P: filepath.Join(root, "elsewhere", "f")}
		if err := ChangePaths(&obj, map[string]string{src: dest}); err == nil {
			t.Fatal("expected an error for a path with no matching root")
		}
	})

	t.Run("nested source roots are rejected", func(t *testing.T) {
		obj := paths{P: filepath.Join(src, "f")}
		mapping := map[string]string{src: dest, filepath.Join(src, "child"): filepath.Join(dest, "child")}
		if err := ChangePaths(&obj, mapping); err == nil {
			t.Fatal("expected an error when one source root is inside another")
		}
	})
}
