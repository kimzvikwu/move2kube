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

package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestIsRemotePath(t *testing.T) {
	tests := map[string]bool{
		"http://example.com/a.yaml":  true,
		"https://example.com/a.yaml": true,
		"ftp://example.com/a.yaml":   false,
		"/local/path.yaml":           false,
		"relative/http://x":          false,
	}
	for in, want := range tests {
		if got := IsRemotePath(in); got != want {
			t.Errorf("IsRemotePath(%q) = %v, want %v", in, got, want)
		}
	}
}

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("content of " + r.URL.Path))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTPContentDownload(t *testing.T) {
	srv := newServer(t)

	t.Run("downloads to the destination", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "out.yaml")
		c := HTTPContent{}
		got, err := c.Download(DownloadOptions{ContentURL: srv.URL + "/a.yaml", DownloadDestinationPath: dest})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != dest || c.ContentFilePath != dest {
			t.Fatalf("want path %q, got %q (ContentFilePath %q)", dest, got, c.ContentFilePath)
		}
		if data, _ := os.ReadFile(dest); string(data) != "content of /a.yaml" {
			t.Fatalf("unexpected file content %q", data)
		}
	})

	t.Run("existing file is kept without overwrite", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "out.yaml")
		if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		c := HTTPContent{}
		if _, err := c.Download(DownloadOptions{ContentURL: srv.URL + "/a.yaml", DownloadDestinationPath: dest}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data, _ := os.ReadFile(dest); string(data) != "old" {
			t.Fatalf("file should not be replaced, got %q", data)
		}
	})

	t.Run("existing file is replaced with overwrite", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "out.yaml")
		if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		c := HTTPContent{}
		if _, err := c.Download(DownloadOptions{ContentURL: srv.URL + "/b.yaml", DownloadDestinationPath: dest, Overwrite: true}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data, _ := os.ReadFile(dest); string(data) != "content of /b.yaml" {
			t.Fatalf("file should be replaced, got %q", data)
		}
	})

	t.Run("non 200 status is an error", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "out.yaml")
		c := HTTPContent{}
		if _, err := c.Download(DownloadOptions{ContentURL: srv.URL + "/missing", DownloadDestinationPath: dest}); err == nil {
			t.Fatal("expected an error for a 404")
		}
	})

	t.Run("empty destination is an error", func(t *testing.T) {
		c := HTTPContent{}
		if _, err := c.Download(DownloadOptions{ContentURL: srv.URL + "/a.yaml"}); err == nil {
			t.Fatal("expected an error for an empty destination")
		}
	})

	t.Run("unreachable server is an error", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "out.yaml")
		c := HTTPContent{}
		if _, err := c.Download(DownloadOptions{ContentURL: "http://127.0.0.1:1/a.yaml", DownloadDestinationPath: dest}); err == nil {
			t.Fatal("expected a connection error")
		}
	})
}

func TestGetDownloadedPath(t *testing.T) {
	srv := newServer(t)
	dest := filepath.Join(t.TempDir(), "cfg.yaml")
	if got := GetDownloadedPath(srv.URL+"/cfg.yaml", dest, true); got != dest {
		t.Fatalf("want %q, got %q", dest, got)
	}
}
