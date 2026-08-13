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
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

type dingTalkRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn dingTalkRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func newDingTalkTestResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestDingTalkGetUserInfoCombinesStableIdentityAndCorpDetails(t *testing.T) {
	idp := NewDingTalkIdProvider("client-id", "client-secret", "https://example.com/callback")
	idp.SetHttpClient(&http.Client{Transport: dingTalkRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "api.dingtalk.com/v1.0/contact/users/me":
			return newDingTalkTestResponse(http.StatusOK, `{"nick":"Alice","openId":"open-id","unionId":"personal-union","email":"personal@example.com","mobile":"13000000000"}`), nil
		case "api.dingtalk.com/v1.0/oauth2/accessToken":
			return newDingTalkTestResponse(http.StatusOK, `{"accessToken":"corp-token"}`), nil
		case "oapi.dingtalk.com/topapi/user/getbyunionid":
			return newDingTalkTestResponse(http.StatusOK, `{"errcode":0,"errmsg":"ok","result":{"userid":"corp-user-id"}}`), nil
		case "oapi.dingtalk.com/topapi/v2/user/get":
			return newDingTalkTestResponse(http.StatusOK, `{"errmsg":"ok","result":{"mobile":"13100000000","email":"corp@example.com","unionid":"corp-union","title":"Engineer"}}`), nil
		default:
			t.Fatalf("unexpected DingTalk request: %s %s", request.Method, request.URL)
			return nil, nil
		}
	})})

	userInfo, err := idp.GetUserInfo(&oauth2.Token{AccessToken: "user-token"})
	if err != nil {
		t.Fatalf("GetUserInfo() error = %v", err)
	}

	if userInfo.Id != "corp-user-id" {
		t.Errorf("Id = %q, want %q", userInfo.Id, "corp-user-id")
	}
	if userInfo.Username != "" {
		t.Errorf("Username = %q, want empty provider login", userInfo.Username)
	}
	if userInfo.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q, want %q", userInfo.DisplayName, "Alice")
	}
	if userInfo.UnionId != "corp-union" {
		t.Errorf("UnionId = %q, want %q", userInfo.UnionId, "corp-union")
	}
	if userInfo.Email != "corp@example.com" {
		t.Errorf("Email = %q, want %q", userInfo.Email, "corp@example.com")
	}
	if userInfo.Phone != "13100000000" {
		t.Errorf("Phone = %q, want %q", userInfo.Phone, "13100000000")
	}
	if userInfo.Extra["title"] != "Engineer" {
		t.Errorf("Extra[title] = %q, want %q", userInfo.Extra["title"], "Engineer")
	}
}

func TestDingTalkGetUserInfoPropagatesCorpTokenError(t *testing.T) {
	idp := NewDingTalkIdProvider("client-id", "client-secret", "https://example.com/callback")
	idp.SetHttpClient(&http.Client{Transport: dingTalkRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "api.dingtalk.com/v1.0/contact/users/me":
			return newDingTalkTestResponse(http.StatusOK, `{"nick":"Alice","openId":"open-id","unionId":"personal-union"}`), nil
		case "api.dingtalk.com/v1.0/oauth2/accessToken":
			return newDingTalkTestResponse(http.StatusInternalServerError, `{"message":"upstream unavailable"}`), nil
		default:
			t.Fatalf("unexpected DingTalk request: %s %s", request.Method, request.URL)
			return nil, nil
		}
	})})

	_, err := idp.GetUserInfo(&oauth2.Token{AccessToken: "user-token"})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("GetUserInfo() error = %v, want HTTP 500 error", err)
	}
}
