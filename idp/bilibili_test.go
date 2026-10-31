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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type bilibiliRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn bilibiliRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func bilibiliTestClient(handler bilibiliRoundTripFunc) *http.Client {
	return &http.Client{Transport: handler}
}

func bilibiliTestResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestBilibiliGetTokenUsesDocumentedRequestAndAbsoluteExpiry(t *testing.T) {
	idp := NewBilibiliIdProvider("client+id", "secret&value", "https://example.com/callback")
	idp.SetHttpClient(bilibiliTestClient(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Scheme != "https" || request.URL.Host != "api.bilibili.com" || request.URL.Path != "/x/account-oauth2/v1/token" {
			t.Errorf("unexpected token endpoint: %s %s", request.Method, request.URL.Redacted())
		}
		if got := request.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", got)
		}
		for name, want := range map[string]string{"client_id": "client+id", "client_secret": "secret&value", "grant_type": "authorization_code", "code": "code+&=/"} {
			if got := request.URL.Query().Get(name); got != want {
				t.Errorf("token query parameter %s = %q, want %q", name, got, want)
			}
		}
		if request.Body != nil {
			body, _ := io.ReadAll(request.Body)
			if len(body) != 0 {
				t.Errorf("token request body = %q, want empty body", body)
			}
		}
		return bilibiliTestResponse(http.StatusOK, `{"code":0,"message":"0","ttl":1,"data":{"access_token":"access-token","refresh_token":"refresh-token","expires_in":1893456000,"scopes":["USER_INFO"]}}`), nil
	}))

	token, err := idp.GetToken("code+&=/")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access-token" || token.RefreshToken != "refresh-token" || token.Expiry.Unix() != 1893456000 {
		t.Errorf("unexpected token or expiry: %+v", token)
	}
}

func TestBilibiliGetUserInfoUsesSignedHTTPSRequest(t *testing.T) {
	idp := NewBilibiliIdProvider("client-id", "client-secret", "https://example.com/callback")
	var previousNonce string
	idp.SetHttpClient(bilibiliTestClient(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.String() != "https://member.bilibili.com/arcopen/fn/user/account/info" {
			t.Errorf("unexpected userinfo endpoint: %s %s", request.Method, request.URL)
		}
		for name, want := range map[string]string{
			"Accept": "application/json", "Content-Type": "application/json", "Access-Token": "user-token",
			"X-Bili-Accesskeyid": "client-id", "X-Bili-Content-Md5": "d41d8cd98f00b204e9800998ecf8427e",
			"X-Bili-Signature-Method": "HMAC-SHA256", "X-Bili-Signature-Version": "2.0",
		} {
			if got := request.Header.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		nonce := request.Header.Get("X-Bili-Signature-Nonce")
		if nonce == "" || nonce == previousNonce {
			t.Errorf("signature nonce must be nonempty and unique, got %q", nonce)
		}
		previousNonce = nonce
		timestamp := request.Header.Get("X-Bili-Timestamp")
		seconds, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil || time.Since(time.Unix(seconds, 0)).Abs() > time.Minute {
			t.Errorf("invalid signature timestamp: %q", timestamp)
		}
		// Build the signing fixture independently from the production header builder.
		canonical := fmt.Sprintf("x-bili-accesskeyid:client-id\nx-bili-content-md5:d41d8cd98f00b204e9800998ecf8427e\nx-bili-signature-method:HMAC-SHA256\nx-bili-signature-nonce:%s\nx-bili-signature-version:2.0\nx-bili-timestamp:%s", nonce, timestamp)
		mac := hmac.New(sha256.New, []byte("client-secret"))
		_, _ = mac.Write([]byte(canonical))
		if got, want := request.Header.Get("Authorization"), hex.EncodeToString(mac.Sum(nil)); got != want {
			t.Errorf("signature = %q, want %q", got, want)
		}
		return bilibiliTestResponse(http.StatusOK, `{"code":0,"message":"0","ttl":1,"data":{"name":"Bili Nickname","face":"https://example.com/avatar.png","openid":"stable-openid"}}`), nil
	}))

	for range 2 {
		user, err := idp.GetUserInfo(&oauth2.Token{AccessToken: "user-token"})
		if err != nil {
			t.Fatal(err)
		}
		if user.Id != "stable-openid" || user.Username != "" || user.DisplayName != "Bili Nickname" || user.AvatarUrl != "https://example.com/avatar.png" {
			t.Errorf("unexpected user mapping: %+v", user)
		}
	}
}

func TestBilibiliRejectsFailedOrIncompleteResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"http error", http.StatusBadGateway, `{"code":0,"data":{"access_token":"token","openid":"user"}}`, "502"},
		{"provider error", http.StatusOK, `{"code":127007,"message":"permission denied"}`, "127007"},
		{"missing identity", http.StatusOK, `{"code":0,"data":{}}`, "empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idp := NewBilibiliIdProvider("client-id", "client-secret", "https://example.com/callback")
			idp.SetHttpClient(bilibiliTestClient(func(*http.Request) (*http.Response, error) {
				return bilibiliTestResponse(tc.status, tc.body), nil
			}))
			_, tokenErr := idp.GetToken("code")
			_, userErr := idp.GetUserInfo(&oauth2.Token{AccessToken: "token"})
			for _, err := range []error{tokenErr, userErr} {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("error = %v, want %q", err, tc.want)
				}
			}
		})
	}
}

func TestBilibiliConfigUsesAuthorizationPage(t *testing.T) {
	idp := NewBilibiliIdProvider("client-id", "secret", "https://example.com/callback")
	authURL, err := url.Parse(idp.Config.AuthCodeURL("state"))
	if err != nil {
		t.Fatal(err)
	}
	if authURL.Scheme+"://"+authURL.Host+authURL.Path != "https://account.bilibili.com/pc/account-pc/auth/oauth" {
		t.Errorf("unexpected authorization endpoint: %s", authURL)
	}
}
