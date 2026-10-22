/*
 *  Copyright IBM Corporation 2023
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

package common_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/konveyor/move2kube/common"
)

func TestIsParentAfterResolvingSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dir"), common.DefaultDirectoryPermission); err != nil {
		t.Fatalf("failed to create the directory. Error: %q", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatalf("failed to create the symlink. Error: %q", err)
	}
	if err := os.Symlink(filepath.Join(root, "dir"), filepath.Join(root, "inside")); err != nil {
		t.Fatalf("failed to create the symlink. Error: %q", err)
	}
	testCases := []struct {
		name     string
		child    string
		expected bool
	}{
		{"same path", root, true},
		{"regular child", filepath.Join(root, "dir", "file"), true},
		{"child that does not exist yet", filepath.Join(root, "dir", "new", "file"), true},
		{"symlink pointing inside", filepath.Join(root, "inside", "file"), true},
		{"symlink pointing outside", filepath.Join(root, "escape", "file"), false},
		{"parent dir segments", root + "/dir/../../x", false},
		{"unrelated path", outside, false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := common.IsParentAfterResolvingSymlinks(testCase.child, root); actual != testCase.expected {
				t.Errorf("for the child '%s' expected %v but got %v", testCase.child, testCase.expected, actual)
			}
		})
	}
}
