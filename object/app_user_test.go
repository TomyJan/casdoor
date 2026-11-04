// Copyright 2026 The Casdoor Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import "testing"

func TestParseAppUserId(t *testing.T) {
	tests := []struct {
		name         string
		userId       string
		organization string
		application  string
	}{
		{name: "typed", userId: "app/acme/portal", organization: "acme", application: "portal"},
		{name: "legacy", userId: "app/portal", organization: "built-in", application: "portal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			organization, application := ParseAppUserId(tt.userId)
			if organization != tt.organization || application != tt.application {
				t.Fatalf("ParseAppUserId(%q) = (%q, %q), want (%q, %q)", tt.userId, organization, application, tt.organization, tt.application)
			}
		})
	}
}

func TestGetAppUserId(t *testing.T) {
	tests := []struct {
		name        string
		application *Application
		want        string
	}{
		{
			name:        "organization-scoped application",
			application: &Application{Organization: "acme", Name: "portal"},
			want:        "app/acme/portal",
		},
		{
			name:        "dynamic client",
			application: &Application{Name: "generated", Tags: []string{"dcr"}},
			want:        "app-dcr/generated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAppUserId(tt.application); got != tt.want {
				t.Fatalf("GetAppUserId() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalUserIsNotGlobalAdmin(t *testing.T) {
	isGlobalAdmin, err := isUserIdGlobalAdmin("acme/alice")
	if err != nil {
		t.Fatal(err)
	}
	if isGlobalAdmin {
		t.Fatal("ordinary user must not be treated as a global admin")
	}
}
