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

package compose

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/cli/cli/compose/types"
	irtypes "github.com/konveyor/move2kube/types/ir"
)

func TestV3LoaderSkipsFilesOutsideSource(t *testing.T) {
	source := t.TempDir()
	outside := t.TempDir()
	insidePath := filepath.Join(source, "inside.txt")
	outsidePath := filepath.Join(outside, "outside.txt")
	symlinkPath := filepath.Join(source, "symlink.txt")
	for _, path := range []string{insidePath, outsidePath} {
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to write the file. Error: %q", err)
		}
	}
	if err := os.Symlink(outsidePath, symlinkPath); err != nil {
		t.Fatalf("failed to create the symlink. Error: %q", err)
	}
	c := v3Loader{sourceDir: source}
	checkStorages := func(t *testing.T, storages []irtypes.Storage) {
		for _, storage := range storages {
			switch storage.Name {
			case "inside":
				if len(storage.Content) == 0 {
					t.Errorf("expected the file inside the source to be read")
				}
			case "outside", "symlink":
				if len(storage.Content) != 0 {
					t.Errorf("expected the file for '%s' outside the source to not be read", storage.Name)
				}
			}
		}
	}

	t.Run("secrets", func(t *testing.T) {
		checkStorages(t, c.getSecretStorages(map[string]types.SecretConfig{
			"inside":  {File: insidePath},
			"outside": {File: outsidePath},
			"symlink": {File: symlinkPath},
		}))
	})
	t.Run("configs", func(t *testing.T) {
		checkStorages(t, c.getConfigStorages(map[string]types.ConfigObjConfig{
			"inside":  {File: insidePath},
			"outside": {File: outsidePath},
			"symlink": {File: symlinkPath},
		}))
	})
	t.Run("env files", func(t *testing.T) {
		composeFilePath := filepath.Join(source, "docker-compose.yaml")
		parsedComposeFile := map[string]interface{}{
			"services": map[string]interface{}{
				"web": map[string]interface{}{envFile: []interface{}{"inside.txt", outsidePath, "symlink.txt"}},
				"db":  map[string]interface{}{envFile: outsidePath},
			},
		}
		services := removeNonExistentEnvFilesV3(composeFilePath, source, parsedComposeFile)["services"].(map[string]interface{})
		webEnvFiles := services["web"].(map[string]interface{})[envFile].([]interface{})
		if len(webEnvFiles) != 1 || webEnvFiles[0] != "inside.txt" {
			t.Errorf("expected only the env file inside the source to be kept but got %v", webEnvFiles)
		}
		if _, ok := services["db"].(map[string]interface{})[envFile]; ok {
			t.Errorf("expected the env file outside the source to be removed")
		}
	})
}
