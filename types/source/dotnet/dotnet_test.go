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

package dotnet

import (
	"encoding/xml"
	"testing"
)

const sampleCSProj = `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
  </PropertyGroup>
  <PropertyGroup Condition="'$(Configuration)|$(Platform)' == 'Release|AnyCPU'">
    <TargetFrameworkVersion>v4.8</TargetFrameworkVersion>
    <OutputPath>bin\Release\</OutputPath>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Newtonsoft.Json" Version="13.0.1" />
    <Reference Include="System.Web" />
    <Content Include="Scripts\Silverlight.js" />
    <None Include="packages.config" />
  </ItemGroup>
</Project>`

func TestCSProjUnmarshal(t *testing.T) {
	proj := CSProj{}
	if err := xml.Unmarshal([]byte(sampleCSProj), &proj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if proj.Sdk != "Microsoft.NET.Sdk.Web" {
		t.Fatalf("want Sdk attr, got %q", proj.Sdk)
	}
	if len(proj.PropertyGroups) != 2 {
		t.Fatalf("want 2 property groups, got %d", len(proj.PropertyGroups))
	}
	if proj.PropertyGroups[0].TargetFramework != "net6.0" {
		t.Fatalf("want net6.0, got %q", proj.PropertyGroups[0].TargetFramework)
	}
	release := proj.PropertyGroups[1]
	if release.Condition == "" || release.TargetFrameworkVersion != "v4.8" || release.OutputPath != `bin\Release\` {
		t.Fatalf("release property group not parsed: %+v", release)
	}
	if len(proj.ItemGroups) != 1 {
		t.Fatalf("want 1 item group, got %d", len(proj.ItemGroups))
	}
	ig := proj.ItemGroups[0]
	if len(ig.PackageReferences) != 1 || ig.PackageReferences[0].Include != "Newtonsoft.Json" || ig.PackageReferences[0].Version != "13.0.1" {
		t.Fatalf("package references not parsed: %+v", ig.PackageReferences)
	}
	if len(ig.References) != 1 || !WebLib.MatchString(ig.References[0].Include) {
		t.Fatalf("references not parsed: %+v", ig.References)
	}
	if len(ig.Contents) != 1 || !WebSLLib.MatchString(ig.Contents[0].Include) {
		t.Fatalf("contents not parsed: %+v", ig.Contents)
	}
	if len(ig.None) != 1 || ig.None[0].Include != "packages.config" {
		t.Fatalf("none items not parsed: %+v", ig.None)
	}
}

const sampleAppConfig = `<?xml version="1.0" encoding="utf-8"?>
<configuration>
  <appSettings>
    <add key="Port" value="8080" />
    <add key="Mode" value="prod" />
  </appSettings>
  <system.serviceModel>
    <services>
      <service>
        <host>
          <baseAddresses>
            <add baseAddress="http://localhost:8733/Design_Time_Addresses/Svc/" />
          </baseAddresses>
        </host>
      </service>
    </services>
  </system.serviceModel>
</configuration>`

func TestAppConfigUnmarshal(t *testing.T) {
	cfg := AppConfig{}
	if err := xml.Unmarshal([]byte(sampleAppConfig), &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	settings := cfg.AppCfgSettings.AddList
	if len(settings) != 2 || settings[0].Key != "Port" || settings[0].Value != "8080" {
		t.Fatalf("app settings not parsed: %+v", settings)
	}
	services := cfg.Model.Services.ServiceList
	if len(services) != 1 {
		t.Fatalf("want 1 service, got %d", len(services))
	}
	addrs := services[0].Host.BaseAddresses.AddList
	if len(addrs) != 1 || addrs[0].BaseAddress != "http://localhost:8733/Design_Time_Addresses/Svc/" {
		t.Fatalf("base addresses not parsed: %+v", addrs)
	}
}

func TestRegexes(t *testing.T) {
	tests := []struct {
		name  string
		match func(string) bool
		in    string
		want  bool
	}{
		{"v4 framework", Version4And3_5.MatchString, "v4.7.2", true},
		{"v3.5 framework", Version4And3_5.MatchString, "v3.5", true},
		{"v2 framework", Version4And3_5.MatchString, "v2.0", false},
		{"System.Web lib", WebLib.MatchString, "System.Web.Mvc", true},
		{"System.Web must be a prefix", WebLib.MatchString, "My.System.Web", false},
		{"AspNet lib", AspNetWebLib.MatchString, "Microsoft.AspNet.Mvc", true},
		{"Silverlight", WebSLLib.MatchString, `Scripts\Silverlight.js`, true},
		{"not Silverlight", WebSLLib.MatchString, "app.js", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.match(tt.in); got != tt.want {
				t.Fatalf("match(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestProjBlockRegex(t *testing.T) {
	sln := `Microsoft Visual Studio Solution File, Format Version 12.00
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "WebApp", "WebApp\WebApp.csproj", "{11111111-1111-1111-1111-111111111111}"
EndProject
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "Lib", "src\Lib\Lib.csproj", "{22222222-2222-2222-2222-222222222222}"
EndProject
Global
EndGlobal
`
	matches := ProjBlockRegex.FindAllStringSubmatch(sln, -1)
	if len(matches) != 2 {
		t.Fatalf("want 2 project blocks, got %d: %v", len(matches), matches)
	}
	if matches[0][1] != `WebApp\WebApp.csproj` || matches[1][1] != `src\Lib\Lib.csproj` {
		t.Fatalf("unexpected project paths: %q, %q", matches[0][1], matches[1][1])
	}
}

func TestPropertiesUnmarshal(t *testing.T) {
	type wrapper struct {
		XMLName    xml.Name    `xml:"PropertyGroup"`
		Properties *Properties `xml:"properties"`
	}
	in := `<PropertyGroup><properties><Foo>1</Foo><Bar>two</Bar></properties></PropertyGroup>`
	out := wrapper{}
	if err := xml.Unmarshal([]byte(in), &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.Properties == nil || out.Properties.Entries["Foo"] != "1" || out.Properties.Entries["Bar"] != "two" {
		t.Fatalf("properties not parsed: %+v", out.Properties)
	}
}
