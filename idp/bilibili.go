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

package idp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/casdoor/casdoor/util"
	"golang.org/x/oauth2"
)

type BilibiliIdProvider struct {
	Client *http.Client
	Config *oauth2.Config
}

func NewBilibiliIdProvider(clientId string, clientSecret string, redirectUrl string) *BilibiliIdProvider {
	idp := &BilibiliIdProvider{}

	config := idp.getConfig(clientId, clientSecret, redirectUrl)
	idp.Config = config

	return idp
}

func (idp *BilibiliIdProvider) SetHttpClient(client *http.Client) {
	idp.Client = client
}

// getConfig return a point of Config, which describes a typical 3-legged OAuth2 flow
func (idp *BilibiliIdProvider) getConfig(clientId string, clientSecret string, redirectUrl string) *oauth2.Config {
	endpoint := oauth2.Endpoint{
		TokenURL: "https://api.bilibili.com/x/account-oauth2/v1/token",
		AuthURL:  "https://account.bilibili.com/pc/account-pc/auth/oauth",
	}

	config := &oauth2.Config{
		Endpoint:     endpoint,
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUrl,
	}

	return config
}

type BilibiliProviderToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type BilibiliIdProviderTokenResponse struct {
	Code    int                   `json:"code"`
	Message string                `json:"message"`
	TTL     int                   `json:"ttl"`
	Data    BilibiliProviderToken `json:"data"`
}

// GetToken
/*
{
    "code": 0,
    "message": "0",
    "ttl": 1,
    "data": {
         "access_token": "d30bedaa4d8eb3128cf35ddc1030e27d",
         "expires_in": 1630220614,
         "refresh_token": "WxFDKwqScZIQDm4iWmKDvetyFugM6HkX"
    }
}
*/
// GetToken use code get access_token (*operation of getting code ought to be done in front)
// get more detail via: https://openhome.bilibili.com/doc/4/eaf0e2b5-bde9-b9a0-9be1-019bb455701c
func (idp *BilibiliIdProvider) GetToken(code string) (*oauth2.Token, error) {
	// Bilibili requires URL parameters and an empty form POST, not a JSON body.
	params := url.Values{
		"client_id":     {idp.Config.ClientID},
		"client_secret": {idp.Config.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
	}
	request, err := http.NewRequest(http.MethodPost, idp.Config.Endpoint.TokenURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := idp.Client.Do(request)
	if err != nil {
		// net/url.Error includes the request URL, which contains the client secret.
		if urlErr, ok := err.(*url.Error); ok {
			return nil, fmt.Errorf("Bilibili token request failed: %w", urlErr.Err)
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bilibili token request failed: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	response := &BilibiliIdProviderTokenResponse{}
	err = json.Unmarshal(data, response)
	if err != nil {
		return nil, err
	}

	if response.Code != 0 {
		return nil, fmt.Errorf("pToken.Errcode = %d, pToken.Errmsg = %s", response.Code, response.Message)
	}
	if response.Data.AccessToken == "" {
		return nil, fmt.Errorf("Bilibili returned an empty access token")
	}

	token := &oauth2.Token{
		AccessToken: response.Data.AccessToken,
		// Despite its name, expires_in is an absolute Unix timestamp in Bilibili's API.
		Expiry:       time.Unix(int64(response.Data.ExpiresIn), 0),
		RefreshToken: response.Data.RefreshToken,
	}

	return token, nil
}

/*
{
    "code": 0,
    "message": "0",
    "ttl": 1,
    "data": {
        "name":"bilibili",
        "face":"http://i0.hdslb.com/bfs/face/e1c99895a9f9df4f260a70dc7e227bcb46cf319c.jpg",
        "openid":"9205eeaa1879skxys969ed47874f225c3"
    }
}
*/

type BilibiliUserInfo struct {
	Name   string `json:"name"`
	Face   string `json:"face"`
	OpenId string `json:"openid"`
}

type BilibiliUserInfoResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	TTL     int              `json:"ttl"`
	Data    BilibiliUserInfo `json:"data"`
}

// GetUserInfo Use  access_token to get UserInfo
// get more detail via: https://openhome.bilibili.com/doc/4/feb66f99-7d87-c206-00e7-d84164cd701c
func (idp *BilibiliIdProvider) GetUserInfo(token *oauth2.Token) (*UserInfo, error) {
	request, err := http.NewRequest(http.MethodGet, "https://member.bilibili.com/arcopen/fn/user/account/info", nil)
	if err != nil {
		return nil, err
	}
	idp.signUserInfoRequest(request, token.AccessToken)
	resp, err := idp.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bilibili userinfo request failed: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	bUserInfoResponse := &BilibiliUserInfoResponse{}
	if err = json.Unmarshal(data, bUserInfoResponse); err != nil {
		return nil, err
	}

	if bUserInfoResponse.Code != 0 {
		return nil, fmt.Errorf("userinfo.Errcode = %d, userinfo.Errmsg = %s", bUserInfoResponse.Code, bUserInfoResponse.Message)
	}

	openID := bUserInfoResponse.Data.OpenId
	if openID == "" {
		return nil, fmt.Errorf("Bilibili returned an empty openid")
	}
	idStr := oauthStableID("", "", openID, "", "")
	userInfo := &UserInfo{
		Id:          idStr,
		Username:    oauthUsernamePreferLogin("", "", "", openID, ""),
		DisplayName: displayNameFromNickname(bUserInfoResponse.Data.Name, "", "", "", idStr),
		AvatarUrl:   bUserInfoResponse.Data.Face,
	}

	return userInfo, nil
}

// https://openhome.bilibili.com/doc/4/8673959e-f7bb-56e6-6e68-d225f971b81b
func (idp *BilibiliIdProvider) signUserInfoRequest(request *http.Request, accessToken string) {
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Access-Token", accessToken)
	request.Header.Set("X-Bili-Accesskeyid", idp.Config.ClientID)
	request.Header.Set("X-Bili-Content-Md5", "d41d8cd98f00b204e9800998ecf8427e")
	request.Header.Set("X-Bili-Signature-Method", "HMAC-SHA256")
	request.Header.Set("X-Bili-Signature-Nonce", util.GenerateUUID())
	request.Header.Set("X-Bili-Signature-Version", "2.0")
	request.Header.Set("X-Bili-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))

	// Sign lowercase header names in lexicographic order, without a trailing newline.
	var lines []string
	for _, name := range []string{"x-bili-accesskeyid", "x-bili-content-md5", "x-bili-signature-method", "x-bili-signature-nonce", "x-bili-signature-version", "x-bili-timestamp"} {
		lines = append(lines, name+":"+request.Header.Get(name))
	}
	mac := hmac.New(sha256.New, []byte(idp.Config.ClientSecret))
	_, _ = mac.Write([]byte(strings.Join(lines, "\n")))
	request.Header.Set("Authorization", hex.EncodeToString(mac.Sum(nil)))
}
