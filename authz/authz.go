// Copyright 2021 The Casdoor Authors. All Rights Reserved.
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
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/object"
	"github.com/casdoor/casdoor/util"
)

var Enforcer *casbin.Enforcer

// defaultApiRules lists the Casbin policy rules that Casdoor requires to function.
// Rules are applied additively: rules already present in the database are left
// untouched, so operator customisations survive restarts and version upgrades
// still pick up any newly added entries here.
//
// NOTE: the former "p, app, *, *, *, *, *" wildcard is intentionally absent.
// Per-organisation scoping for application credentials is enforced in IsAllowed,
// so that Casbin rule was both redundant and dangerously misleading.
var defaultApiRules = [][]string{
	{"built-in", "*", "*", "*", "*", "*"},
	{"app-dcr", "*", "*", "/api/login/oauth/*", "*", "*"},
	{"app-dcr", "*", "*", "/api/get-oauth-token", "*", "*"},
	{"app-dcr", "*", "*", "/api/userinfo", "*", "*"},
	{"app-dcr", "*", "*", "/api/get-application", "*", "*"},
	{"*", "*", "POST", "/api/signup", "*", "*"},
	{"*", "*", "GET", "/api/get-email-and-phone", "*", "*"},
	{"*", "*", "POST", "/api/login", "*", "*"},
	{"*", "*", "GET", "/api/get-app-login", "*", "*"},
	{"*", "*", "POST", "/api/logout", "*", "*"},
	{"*", "*", "GET", "/api/logout", "*", "*"},
	{"*", "*", "POST", "/api/sso-logout", "*", "*"},
	{"*", "*", "GET", "/api/sso-logout", "*", "*"},
	{"*", "*", "POST", "/api/callback", "*", "*"},
	{"*", "*", "POST", "/api/device-auth", "*", "*"},
	{"*", "*", "POST", "/api/cancel-device-auth", "*", "*"},
	{"*", "*", "POST", "/api/device-auth-complete", "*", "*"},
	{"*", "*", "POST", "/api/native-sso-complete", "*", "*"},
	{"*", "*", "GET", "/api/get-account", "*", "*"},
	{"*", "*", "GET", "/api/userinfo", "*", "*"},
	{"*", "*", "GET", "/api/user", "*", "*"},
	{"*", "*", "GET", "/api/health", "*", "*"},
	{"*", "*", "*", "/api/webhook", "*", "*"},
	{"*", "*", "GET", "/api/get-qrcode", "*", "*"},
	{"*", "*", "GET", "/api/get-webhook-event", "*", "*"},
	{"*", "*", "GET", "/api/get-captcha-status", "*", "*"},
	{"*", "*", "*", "/api/login/oauth", "*", "*"},
	{"*", "*", "POST", "/api/oauth/register", "*", "*"},
	{"*", "*", "GET", "/api/get-application", "*", "*"},
	{"*", "*", "GET", "/api/get-organization-applications", "*", "*"},
	{"*", "*", "GET", "/api/get-user", "*", "*"},
	{"*", "*", "GET", "/api/get-user-application", "*", "*"},
	{"*", "*", "POST", "/api/upload-users", "*", "*"},
	{"*", "*", "GET", "/api/get-resources", "*", "*"},
	{"*", "*", "GET", "/api/get-records", "*", "*"},
	{"*", "*", "GET", "/api/get-product", "*", "*"},
	{"*", "*", "GET", "/api/get-products", "*", "*"},
	{"*", "*", "POST", "/api/buy-product", "*", "*"},
	{"*", "*", "GET", "/api/get-order", "*", "*"},
	{"*", "*", "GET", "/api/get-orders", "*", "*"},
	{"*", "*", "GET", "/api/get-user-orders", "*", "*"},
	{"*", "*", "GET", "/api/get-payment", "*", "*"},
	{"*", "*", "POST", "/api/invoice-payment", "*", "*"},
	{"*", "*", "POST", "/api/notify-payment", "*", "*"},
	{"*", "*", "POST", "/api/place-order", "*", "*"},
	{"*", "*", "POST", "/api/cancel-order", "*", "*"},
	{"*", "*", "POST", "/api/pay-order", "*", "*"},
	{"*", "*", "POST", "/api/validate-coupon", "*", "*"},
	{"*", "*", "POST", "/api/unlink", "*", "*"},
	{"*", "*", "POST", "/api/set-password", "*", "*"},
	{"*", "*", "POST", "/api/send-verification-code", "*", "*"},
	{"*", "*", "GET", "/api/get-captcha", "*", "*"},
	{"*", "*", "POST", "/api/verify-captcha", "*", "*"},
	{"*", "*", "POST", "/api/verify-code", "*", "*"},
	{"*", "*", "POST", "/api/v1/traces", "*", "*"},
	{"*", "*", "POST", "/api/v1/metrics", "*", "*"},
	{"*", "*", "POST", "/api/v1/logs", "*", "*"},
	{"*", "*", "POST", "/api/reset-email-or-phone", "*", "*"},
	{"*", "*", "POST", "/api/upload-resource", "*", "*"},
	{"*", "*", "GET", "/.well-known/openid-configuration", "*", "*"},
	{"*", "*", "GET", "/.well-known/oauth-authorization-server", "*", "*"},
	{"*", "*", "GET", "/.well-known/oauth-protected-resource", "*", "*"},
	{"*", "*", "GET", "/.well-known/webfinger", "*", "*"},
	{"*", "*", "*", "/.well-known/jwks", "*", "*"},
	{"*", "*", "GET", "/.well-known/:application/openid-configuration", "*", "*"},
	{"*", "*", "GET", "/.well-known/:application/oauth-authorization-server", "*", "*"},
	{"*", "*", "GET", "/.well-known/:application/oauth-protected-resource", "*", "*"},
	{"*", "*", "GET", "/.well-known/:application/webfinger", "*", "*"},
	{"*", "*", "*", "/.well-known/:application/jwks", "*", "*"},
	{"*", "*", "GET", "/api/get-saml-login", "*", "*"},
	{"*", "*", "POST", "/api/acs", "*", "*"},
	{"*", "*", "GET", "/api/saml/metadata", "*", "*"},
	{"*", "*", "*", "/api/saml/redirect", "*", "*"},
	{"*", "*", "*", "/cas", "*", "*"},
	{"*", "*", "*", "/scim", "*", "*"},
	{"*", "*", "*", "/api/webauthn", "*", "*"},
	{"*", "*", "GET", "/api/get-release", "*", "*"},
	{"*", "*", "GET", "/api/get-default-application", "*", "*"},
	{"*", "*", "GET", "/api/get-prometheus-info", "*", "*"},
	{"*", "*", "*", "/api/metrics", "*", "*"},
	{"*", "*", "GET", "/api/get-pricing", "*", "*"},
	{"*", "*", "GET", "/api/get-plan", "*", "*"},
	{"*", "*", "GET", "/api/get-subscription", "*", "*"},
	{"*", "*", "GET", "/api/get-transactions", "*", "*"},
	{"*", "*", "GET", "/api/get-transaction", "*", "*"},
	{"*", "*", "GET", "/api/get-provider", "*", "*"},
	{"*", "*", "GET", "/api/get-organization-names", "*", "*"},
	{"*", "*", "GET", "/api/get-organizations", "*", "*"},
	{"*", "*", "GET", "/api/get-all-objects", "*", "*"},
	{"*", "*", "GET", "/api/get-all-actions", "*", "*"},
	{"*", "*", "GET", "/api/get-all-roles", "*", "*"},
	{"*", "*", "GET", "/api/run-casbin-command", "*", "*"},
	{"*", "*", "POST", "/api/refresh-engines", "*", "*"},
	{"*", "*", "GET", "/api/get-invitation-info", "*", "*"},
	{"*", "*", "GET", "/api/faceid-signin-begin", "*", "*"},
	{"*", "*", "GET", "/api/kerberos-login", "*", "*"},
}

// obsoleteApiRules lists rules that must be removed from the database if present.
// These were previously seeded as defaults but are now either replaced by
// code-level enforcement or were incorrectly over-permissive.
var obsoleteApiRules = [][]string{
	// Superseded by org-scoped enforcement in IsAllowed; keeping this in the
	// DB gives a false impression that app credentials are unrestricted.
	{"app", "*", "*", "*", "*", "*"},
}

func InitApi() {
	e, err := object.GetInitializedEnforcer(util.GetId("built-in", "api-enforcer-built-in"))
	if err != nil {
		panic(err)
	}

	Enforcer = e.Enforcer
	// GetInitializedEnforcer already loaded rules from the DB into memory.
	// We use an additive strategy: only insert rules that are not yet present,
	// so that operator customisations survive restarts and new default rules
	// introduced in upgrades are still picked up automatically.

	// 1. Remove rules that have been superseded or were incorrectly permissive.
	for _, rule := range obsoleteApiRules {
		params := make([]interface{}, len(rule))
		for i, v := range rule {
			params[i] = v
		}
		if Enforcer.HasPolicy(params...) {
			if _, err = Enforcer.RemovePolicy(params...); err != nil {
				panic(err)
			}
		}
	}

	// 2. Add any default rules that are missing from the DB.
	var missing [][]string
	for _, rule := range defaultApiRules {
		params := make([]interface{}, len(rule))
		for i, v := range rule {
			params[i] = v
		}
		if !Enforcer.HasPolicy(params...) {
			missing = append(missing, rule)
		}
	}
	if len(missing) > 0 {
		if _, err = Enforcer.AddPolicies(missing); err != nil {
			panic(err)
		}
	}

	if err = Enforcer.SavePolicy(); err != nil {
		panic(err)
	}
}

func IsAllowed(subOwner string, subName string, method string, urlPath string, objOwner string, objName string, extraInfo map[string]interface{}) (bool, error) {
	if urlPath == "/api/mcp" {
		if detailPath, ok := extraInfo["detailPathUrl"].(string); ok {
			if detailPath == "initialize" || detailPath == "notifications/initialized" || detailPath == "ping" || detailPath == "tools/list" {
				return true, nil
			}
		}
	}

	if conf.IsDemoMode() {
		if !isAllowedInDemoMode(subOwner, subName, method, urlPath, objOwner, objName) {
			return false, nil
		}
	}

	if subOwner == "app" {
		// subName is "{appOrg}/{appName}" (new) or "{appName}" (legacy, treated as built-in).
		// Built-in org apps retain global-admin access; others are scoped to their own org.
		appOrg, _ := object.ParseAppUserId("app/" + subName)
		if appOrg == "built-in" || objOwner == "" || appOrg == objOwner {
			return true, nil
		}
	} else {
		user, err := object.GetUser(util.GetId(subOwner, subName))
		if err != nil {
			return false, err
		}

		if user != nil {
			if user.IsDeleted {
				return false, nil
			}

			if user.IsGlobalAdmin() {
				return true, nil
			}

			if user.IsAdmin && subOwner == objOwner {
				return true, nil
			}
		}
	}

	res, err := Enforcer.Enforce(subOwner, subName, method, urlPath, objOwner, objName)
	if err != nil {
		return false, err
	}

	if !res {
		res, err = object.CheckApiPermission(util.GetId(subOwner, subName), objOwner, urlPath, method)
		if err != nil {
			return false, err
		}
	}

	return res, nil
}

func isAllowedInDemoMode(subOwner string, subName string, method string, urlPath string, objOwner string, objName string) bool {
	if method == "POST" {
		if strings.HasPrefix(urlPath, "/api/login") || urlPath == "/api/logout" || urlPath == "/api/sso-logout" || urlPath == "/api/signup" || urlPath == "/api/callback" || urlPath == "/api/send-verification-code" || urlPath == "/api/send-email" || urlPath == "/api/verify-captcha" || urlPath == "/api/verify-code" || urlPath == "/api/check-user-password" || strings.HasPrefix(urlPath, "/api/mfa/") || urlPath == "/api/webhook" || urlPath == "/api/get-qrcode" || urlPath == "/api/refresh-engines" {
			return true
		} else if urlPath == "/api/update-user" {
			// Allow ordinary users to update their own information
			if (subOwner == objOwner && subName == objName || subOwner == "app") && !(subOwner == "built-in" && subName == "admin") {
				return true
			}
			return false
		} else if urlPath == "/api/upload-resource" || urlPath == "/api/add-transaction" {
			if subOwner == "app" && subName == "app-casibase" {
				return true
			}
			return false
		} else {
			return false
		}
	}

	// If method equals GET
	return true
}
