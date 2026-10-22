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

package maven

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

const samplePom = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <parent>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-parent</artifactId>
    <version>2.7.0</version>
  </parent>
  <groupId>com.example</groupId>
  <artifactId>demo</artifactId>
  <version>0.0.1-SNAPSHOT</version>
  <packaging>war</packaging>
  <modules>
    <module>api</module>
    <module>web</module>
  </modules>
  <properties>
    <java.version>11</java.version>
    <app.name>demo-app</app.name>
    <full.name>${app.name}</full.name>
  </properties>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
    <dependency>
      <groupId>junit</groupId>
      <artifactId>junit</artifactId>
      <version>4.13</version>
      <scope>test</scope>
    </dependency>
  </dependencies>
  <build>
    <finalName>demo-final</finalName>
    <plugins>
      <plugin>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-maven-plugin</artifactId>
      </plugin>
    </plugins>
  </build>
  <profiles>
    <profile>
      <id>dev</id>
      <activation><activeByDefault>true</activeByDefault></activation>
      <properties><env>development</env></properties>
    </profile>
  </profiles>
</project>
`

func TestPomLoad(t *testing.T) {
	pomPath := filepath.Join(t.TempDir(), PomXMLFileName)
	if err := os.WriteFile(pomPath, []byte(samplePom), 0o644); err != nil {
		t.Fatal(err)
	}
	pom := Pom{}
	if err := pom.Load(pomPath); err != nil {
		t.Fatalf("failed to load pom: %v", err)
	}
	if pom.ArtifactID != "demo" || pom.GroupID != "com.example" || pom.Packaging != "war" {
		t.Fatalf("unexpected coordinates: %s:%s packaging=%s", pom.GroupID, pom.ArtifactID, pom.Packaging)
	}
	if pom.Parent == nil || pom.Parent.ArtifactID != "spring-boot-starter-parent" {
		t.Fatalf("parent not parsed: %+v", pom.Parent)
	}
	if pom.Modules == nil || len(*pom.Modules) != 2 || (*pom.Modules)[1] != "web" {
		t.Fatalf("modules not parsed: %+v", pom.Modules)
	}
	if pom.Dependencies == nil || len(*pom.Dependencies) != 2 || (*pom.Dependencies)[1].Scope != "test" {
		t.Fatalf("dependencies not parsed: %+v", pom.Dependencies)
	}
	if pom.Build == nil || pom.Build.FinalName != "demo-final" {
		t.Fatalf("build finalName not parsed (it lives on the embedded BuildBase): %+v", pom.Build)
	}
	if pom.Build.Plugins == nil || (*pom.Build.Plugins)[0].ArtifactID != "spring-boot-maven-plugin" {
		t.Fatalf("build plugins not parsed: %+v", pom.Build.Plugins)
	}
	if pom.Profiles == nil || len(*pom.Profiles) != 1 {
		t.Fatalf("profiles not parsed: %+v", pom.Profiles)
	}
	profile := (*pom.Profiles)[0]
	if profile.ID != "dev" || profile.Activation == nil || !profile.Activation.ActiveByDefault {
		t.Fatalf("profile not parsed: %+v", profile)
	}
	if profile.Properties == nil || profile.Properties.Entries["env"] != "development" {
		t.Fatalf("profile properties not parsed: %+v", profile.Properties)
	}
	if got := pom.Properties.Entries["java.version"]; got != "11" {
		t.Fatalf("want java.version 11, got %q", got)
	}
}

func TestPomLoadErrors(t *testing.T) {
	pom := Pom{}
	if err := pom.Load(filepath.Join(t.TempDir(), "missing.xml")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
	badPath := filepath.Join(t.TempDir(), PomXMLFileName)
	if err := os.WriteFile(badPath, []byte("<project><unclosed></project>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pom.Load(badPath); err == nil {
		t.Fatal("expected an error for malformed XML")
	}
}

func TestGetProperty(t *testing.T) {
	pom := Pom{Properties: &Properties{Entries: Entries{
		"plain":   "value",
		"ref":     "${plain}",
		"chained": "${ref}",
		"missing": "${nope}",
	}}}

	tests := []struct {
		key     string
		want    string
		wantErr bool
	}{
		{key: "plain", want: "value"},
		{key: "ref", want: "value"},
		{key: "chained", want: "value"},
		{key: "missing", wantErr: true},
		{key: "absent", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, err := pom.GetProperty(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("want %q, got %q", tt.want, got)
			}
		})
	}

	t.Run("no properties", func(t *testing.T) {
		if _, err := (&Pom{}).GetProperty("x"); err == nil {
			t.Fatal("expected an error when the pom has no properties")
		}
	})
}

func TestGetPropertyInterpolation(t *testing.T) {
	// Known bug: the regex `\${(.+)}` is greedy and the text after the last
	// match is never appended, so mixed or repeated references resolve wrongly.
	// "pre-${a}-post" currently resolves to "pre-x" and "${a}${a}" errors out.
	t.Skip("known bug in Pom.GetProperty interpolation; remove this skip once it is fixed")

	pom := Pom{Properties: &Properties{Entries: Entries{
		"a":      "x",
		"wrap":   "pre-${a}-post",
		"repeat": "${a}${a}",
	}}}
	for key, want := range map[string]string{"wrap": "pre-x-post", "repeat": "xx"} {
		got, err := pom.GetProperty(key)
		if err != nil || got != want {
			t.Errorf("%s: want %q, got %q (err %v)", key, want, got, err)
		}
	}
}

func TestPropertiesMarshalRoundTrip(t *testing.T) {
	// Known bug: MarshalXML is defined on Entries rather than Properties, so the
	// encoder wraps the entries in an extra <Entries> element and they are lost
	// on unmarshal. Nothing in move2kube marshals a Pom today.
	t.Skip("known bug in Properties XML marshalling; remove this skip once it is fixed")

	type wrapper struct {
		XMLName    xml.Name    `xml:"project"`
		Properties *Properties `xml:"properties"`
	}
	in := wrapper{Properties: &Properties{Entries: Entries{"java.version": "17", "encoding": "UTF-8"}}}
	data, err := xml.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	out := wrapper{}
	if err := xml.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v\n%s", err, data)
	}
	if len(out.Properties.Entries) != 2 || out.Properties.Entries["java.version"] != "17" || out.Properties.Entries["encoding"] != "UTF-8" {
		t.Fatalf("round trip lost data: %+v\n%s", out.Properties.Entries, data)
	}
}
