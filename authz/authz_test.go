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

package authz

import (
	"slices"
	"testing"
)

func TestDefaultAndObsoleteApiRules(t *testing.T) {
	newRules := [][]string{
		{"*", "*", "POST", "/api/userinfo", "*", "*"},
		{"*", "*", "GET", "/api/get-init-admin-status", "*", "*"},
		{"*", "*", "POST", "/api/init-admin-password", "*", "*"},
	}
	for _, rule := range newRules {
		if !containsRule(defaultApiRules, rule) {
			t.Errorf("defaultApiRules is missing %v", rule)
		}
		if containsRule(obsoleteApiRules, rule) {
			t.Errorf("obsoleteApiRules unexpectedly contains %v", rule)
		}
	}

	removedRules := [][]string{
		{"*", "*", "GET", "/api/run-casbin-command", "*", "*"},
		{"*", "*", "POST", "/api/refresh-engines", "*", "*"},
	}
	for _, rule := range removedRules {
		if containsRule(defaultApiRules, rule) {
			t.Errorf("defaultApiRules still contains removed route %v", rule)
		}
		if !containsRule(obsoleteApiRules, rule) {
			t.Errorf("obsoleteApiRules is missing removed route %v", rule)
		}
	}
}

func containsRule(rules [][]string, target []string) bool {
	return slices.ContainsFunc(rules, func(rule []string) bool {
		return slices.Equal(rule, target)
	})
}
