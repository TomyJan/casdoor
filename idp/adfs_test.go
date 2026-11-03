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

package idp

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestAdfsSetHttpClientDoesNotMutateSharedClient(t *testing.T) {
	sharedTransport := http.DefaultTransport.(*http.Transport).Clone()
	originalTLSConfig := sharedTransport.TLSClientConfig
	sharedClient := &http.Client{
		Transport: sharedTransport,
		Timeout:   time.Second,
	}
	idp := NewAdfsIdProvider("client-id", "client-secret", "https://example.com/callback", "https://adfs.example.com")

	idp.SetHttpClient(sharedClient)

	if idp.Client == sharedClient {
		t.Fatal("SetHttpClient() reused the shared client instead of cloning it")
	}
	if sharedClient.Transport != sharedTransport {
		t.Fatal("SetHttpClient() replaced the shared client's transport")
	}
	if sharedTransport.TLSClientConfig != originalTLSConfig {
		t.Fatal("SetHttpClient() changed the shared transport's TLS configuration")
	}
	providerTransport, ok := idp.Client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("provider transport type = %T, want *http.Transport", idp.Client.Transport)
	}
	if providerTransport.TLSClientConfig == nil || !providerTransport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("provider transport must skip TLS verification for self-signed ADFS deployments")
	}
	if idp.Client.Timeout != sharedClient.Timeout {
		t.Fatalf("provider timeout = %v, want %v", idp.Client.Timeout, sharedClient.Timeout)
	}
}

func TestAdfsGetUserInfoRejectsEmptyDiscoveryKeys(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/adfs/discovery/keys" {
			t.Fatalf("request path = %q, want %q", request.URL.Path, "/adfs/discovery/keys")
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"keys":[]}`))
	}))
	defer server.Close()

	idp := NewAdfsIdProvider("client-id", "client-secret", "https://example.com/callback", server.URL)
	idp.SetHttpClient(&http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
	})

	_, err := idp.GetUserInfo(&oauth2.Token{AccessToken: "not-used"})
	if err == nil || !strings.Contains(err.Error(), "discovery keys are empty") {
		t.Fatalf("GetUserInfo() error = %v, want empty discovery keys error", err)
	}
}
