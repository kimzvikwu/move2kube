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

package knownhosts

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func newPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return sshPub
}

func keyFields(k ssh.PublicKey) string {
	return k.Type() + " " + base64.StdEncoding.EncodeToString(k.Marshal())
}

func writeKnownHosts(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNextWord(t *testing.T) {
	tests := []struct {
		in, word, rest string
	}{
		{"github.com ssh-ed25519 AAAA", "github.com", "ssh-ed25519 AAAA"},
		{"a\tb", "a", "b"},
		{"single", "single", ""},
		{"a    b", "a", "b"},
	}
	for _, tt := range tests {
		w, rest := nextWord([]byte(tt.in))
		if w != tt.word || string(rest) != tt.rest {
			t.Errorf("nextWord(%q) = (%q, %q), want (%q, %q)", tt.in, w, rest, tt.word, tt.rest)
		}
	}
}

func TestParseKnownHostsLine(t *testing.T) {
	key := keyFields(newPublicKey(t))

	t.Run("valid line", func(t *testing.T) {
		line := "github.com " + key + " comment"
		ignore, host, lineStr, err := parseKnownHostsLine([]byte(line))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ignore || host != "github.com" || lineStr != line {
			t.Fatalf("got ignore=%v host=%q line=%q", ignore, host, lineStr)
		}
	})

	for _, marker := range []string{markerCert, markerRevoked} {
		t.Run(marker+" lines are ignored", func(t *testing.T) {
			ignore, host, _, err := parseKnownHostsLine([]byte(marker + " *.example.com " + key))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ignore || host != "*.example.com" {
				t.Fatalf("got ignore=%v host=%q", ignore, host)
			}
		})
	}

	errorCases := map[string]string{
		"missing host pattern": "onlyhost",
		"missing key type":     "host ssh-ed25519",
		"bad base64":           "host ssh-ed25519 !!!not-base64!!!",
		"not a public key":     "host ssh-ed25519 " + base64.StdEncoding.EncodeToString([]byte("garbage")),
	}
	for name, line := range errorCases {
		t.Run(name, func(t *testing.T) {
			if _, _, _, err := parseKnownHostsLine([]byte(line)); err == nil {
				t.Fatalf("expected an error for %q", line)
			}
		})
	}
}

func TestParseKnownHosts(t *testing.T) {
	k1 := keyFields(newPublicKey(t))
	k2 := keyFields(newPublicKey(t))

	t.Run("collects keys per host", func(t *testing.T) {
		path := writeKnownHosts(t,
			"# a comment",
			"",
			"github.com,140.82.112.3 "+k1,
			"gitlab.com "+k1,
			"gitlab.com "+k2,
			"|1|hashedsalt=|hashedhost= "+k1,
			markerRevoked+" bad.example.com "+k2,
		)
		got, err := ParseKnownHosts(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("want 3 hosts (hashed and revoked entries skipped), got %d: %v", len(got), got)
		}
		if len(got["github.com"]) != 1 || len(got["140.82.112.3"]) != 1 {
			t.Fatalf("comma-separated hosts should each get the line: %v", got)
		}
		if len(got["gitlab.com"]) != 2 {
			t.Fatalf("want 2 lines for gitlab.com, got %v", got["gitlab.com"])
		}
		if _, ok := got["bad.example.com"]; ok {
			t.Fatal("revoked entries should be skipped")
		}
	})

	t.Run("invalid line reports the line number", func(t *testing.T) {
		path := writeKnownHosts(t, "github.com "+k1, "broken")
		_, err := ParseKnownHosts(path)
		if err == nil || !strings.Contains(err.Error(), "line no. 2") {
			t.Fatalf("expected an error mentioning line 2, got %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if _, err := ParseKnownHosts(filepath.Join(t.TempDir(), "nope")); err == nil {
			t.Fatal("expected an error for a missing file")
		}
	})
}

func TestAddKeyToMap(t *testing.T) {
	key := newPublicKey(t)
	m := map[string]ssh.PublicKey{}
	cb := addKeyToMap(m)
	if err := cb("example.com:22", &net.TCPAddr{}, key); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["example.com"] == nil || string(m["example.com"].Marshal()) != string(key.Marshal()) {
		t.Fatalf("key was not stored under the host name: %v", m)
	}
	if err := cb("no-port", &net.TCPAddr{}, key); err == nil {
		t.Fatal("expected an error for an address with no port")
	}
}
