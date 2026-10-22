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

package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplicateSymlinks(t *testing.T) {
	source := t.TempDir()
	outside := t.TempDir()
	secretPath := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to write the file. Error: %q", err)
	}
	if err := os.WriteFile(filepath.Join(source, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write the file. Error: %q", err)
	}
	if err := os.Symlink(secretPath, filepath.Join(source, "escape.txt")); err != nil {
		t.Fatalf("failed to create the symlink. Error: %q", err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "escapedir")); err != nil {
		t.Fatalf("failed to create the symlink. Error: %q", err)
	}
	if err := os.Symlink("file.txt", filepath.Join(source, "inside.txt")); err != nil {
		t.Fatalf("failed to create the symlink. Error: %q", err)
	}
	destination := filepath.Join(t.TempDir(), "destination")

	t.Run("symlinks pointing outside the source are skipped and symlinks pointing inside are recreated", func(t *testing.T) {
		// replicate twice to check that existing symlinks at the destination are handled
		for i := 0; i < 2; i++ {
			if err := Replicate(source, destination); err != nil {
				t.Fatalf("failed to replicate. Error: %q", err)
			}
		}
		for _, name := range []string{"escape.txt", "escapedir"} {
			if _, err := os.Lstat(filepath.Join(destination, name)); !os.IsNotExist(err) {
				t.Errorf("expected the symlink '%s' pointing outside the source to be skipped. Error: %v", name, err)
			}
		}
		fi, err := os.Lstat(filepath.Join(destination, "inside.txt"))
		if err != nil {
			t.Fatalf("expected the symlink pointing inside the source to be recreated. Error: %q", err)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected a symlink but got the mode %v", fi.Mode())
		}
		if link, err := os.Readlink(filepath.Join(destination, "inside.txt")); err != nil || link != "file.txt" {
			t.Errorf("expected the symlink to point to 'file.txt' but got '%s'. Error: %v", link, err)
		}
	})

	t.Run("the top level source is followed if it is a symlink", func(t *testing.T) {
		sourceLink := filepath.Join(t.TempDir(), "sourcelink")
		if err := os.Symlink(source, sourceLink); err != nil {
			t.Fatalf("failed to create the symlink. Error: %q", err)
		}
		destination := filepath.Join(t.TempDir(), "destination")
		if err := Replicate(sourceLink, destination); err != nil {
			t.Fatalf("failed to replicate. Error: %q", err)
		}
		data, err := os.ReadFile(filepath.Join(destination, "file.txt"))
		if err != nil || string(data) != "data" {
			t.Errorf("expected the file to be copied. Data: '%s' Error: %v", data, err)
		}
	})
}
