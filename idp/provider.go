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
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

type UserInfo struct {
	Id          string
	Username    string
	DisplayName string
	UnionId     string
	Email       string
	Phone       string
	CountryCode string
	AvatarUrl   string
	Extra       map[string]string
}

type ProviderInfo struct {
	Type          string
	SubType       string
	ClientId      string
	ClientSecret  string
	ClientId2     string
	ClientSecret2 string
	AppId         string
	HostUrl       string
	RedirectUrl   string
	DisableSsl    bool
	CodeVerifier  string

	TokenURL    string
	AuthURL     string
	UserInfoURL string
	UserMapping map[string]string

	AppCertificate  string
	RootCertificate string
}

type IdProvider interface {
	SetHttpClient(client *http.Client)
	GetToken(code string) (*oauth2.Token, error)
	GetUserInfo(token *oauth2.Token) (*UserInfo, error)
}

func GetIdProvider(idpInfo *ProviderInfo, redirectUrl string) (IdProvider, error) {
	switch idpInfo.Type {
	case "GitHub":
		return NewGithubIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "Google":
		return NewGoogleIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "QQ":
		return NewQqIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "WeChat":
		if idpInfo.SubType == "Mobile" {
			return NewWeChatMobileIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
		} else {
			// Default to Web (PC QR code login) for backward compatibility
			return NewWeChatIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
		}
	case "Facebook":
		return NewFacebookIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "DingTalk":
		return NewDingTalkIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "Weibo":
		return NewWeiBoIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "Gitee":
		return NewGiteeIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "LinkedIn":
		return NewLinkedInIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "WeCom":
		if idpInfo.SubType == "Internal" {
			return NewWeComInternalIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.DisableSsl), nil
		} else if idpInfo.SubType == "Third-party" {
			return NewWeComIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.DisableSsl), nil
		} else {
			return nil, fmt.Errorf("WeCom provider subType: %s is not supported", idpInfo.SubType)
		}
	case "Lark":
		return NewLarkIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.DisableSsl), nil
	case "GitLab":
		return NewGitlabIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "ADFS":
		return NewAdfsIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.HostUrl), nil
	case "AzureADB2C":
		return NewAzureAdB2cProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.HostUrl, idpInfo.AppId), nil
	case "Baidu":
		return NewBaiduIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "Alipay":
		return NewAlipayIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.AppCertificate, idpInfo.RootCertificate)
	case "Custom", "Custom Flexible":
		return NewCustomIdProvider(idpInfo, redirectUrl), nil
	case "Infoflow":
		if idpInfo.SubType == "Internal" {
			return NewInfoflowInternalIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, idpInfo.AppId, redirectUrl), nil
		} else if idpInfo.SubType == "Third-party" {
			return NewInfoflowIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, idpInfo.AppId, redirectUrl), nil
		} else {
			return nil, fmt.Errorf("Infoflow provider subType: %s is not supported", idpInfo.SubType)
		}
	case "Casdoor":
		return NewCasdoorIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.HostUrl), nil
	case "Okta":
		return NewOktaIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl, idpInfo.HostUrl), nil
	case "Douyin":
		return NewDouyinIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "Kwai":
		return NewKwaiIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "Bilibili":
		return NewBilibiliIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	case "MetaMask":
		return NewMetaMaskIdProvider(), nil
	case "Web3Onboard":
		return NewWeb3OnboardIdProvider(), nil
	case "Twitter":
		provider := NewTwitterIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl)
		provider.CodeVerifier = idpInfo.CodeVerifier
		return provider, nil
	case "Telegram":
		return NewTelegramIdProvider(idpInfo.ClientId, idpInfo.ClientSecret, redirectUrl), nil
	default:
		if isGothSupport(idpInfo.Type) {
			return NewGothIdProvider(idpInfo.Type, idpInfo.ClientId, idpInfo.ClientSecret, idpInfo.ClientId2, idpInfo.ClientSecret2, redirectUrl, idpInfo.HostUrl)
		}
		if strings.HasPrefix(idpInfo.Type, "Custom") {
			return NewCustomIdProvider(idpInfo, redirectUrl), nil
		}
		return nil, fmt.Errorf("OAuth provider type: %s is not supported", idpInfo.Type)
	}
}

var gothList = []string{
	"Apple",
	"AzureAD",
	"Slack",
	"Steam",
	"Line",
	"Amazon",
	"Auth0",
	"BattleNet",
	"Bitbucket",
	"Box",
	"CloudFoundry",
	"Dailymotion",
	"Deezer",
	"DigitalOcean",
	"Discord",
	"Dropbox",
	"EveOnline",
	"Fitbit",
	"Gitea",
	"Heroku",
	"InfluxCloud",
	"Instagram",
	"Intercom",
	"Kakao",
	"Lastfm",
	"Mailru",
	"Meetup",
	"MicrosoftOnline",
	"Naver",
	"Nextcloud",
	"OneDrive",
	"Oura",
	"Patreon",
	"Paypal",
	"SalesForce",
	"Shopify",
	"Soundcloud",
	"Spotify",
	"Strava",
	"Stripe",
	"TikTok",
	"Tumblr",
	"Twitch",
	"Typetalk",
	"Uber",
	"VK",
	"Wepay",
	"Xero",
	"Yahoo",
	"Yammer",
	"Yandex",
	"Zoom",
}

func isGothSupport(provider string) bool {
	for _, value := range gothList {
		if strings.EqualFold(value, provider) {
			return true
		}
	}
	return false
}

// firstNonEmpty returns the first non-empty string after trimming whitespace.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func rawString(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(val)
	case fmt.Stringer:
		return strings.TrimSpace(val.String())
	case int:
		return fmt.Sprintf("%d", val)
	case int8:
		return fmt.Sprintf("%d", val)
	case int16:
		return fmt.Sprintf("%d", val)
	case int32:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case uint:
		return fmt.Sprintf("%d", val)
	case uint8:
		return fmt.Sprintf("%d", val)
	case uint16:
		return fmt.Sprintf("%d", val)
	case uint32:
		return fmt.Sprintf("%d", val)
	case uint64:
		return fmt.Sprintf("%d", val)
	case float32:
		return strings.TrimSpace(fmt.Sprintf("%.0f", val))
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%.0f", val))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", val))
	}
}

func rawFirstNonEmpty(raw map[string]interface{}, keys ...string) string {
	if raw == nil {
		return ""
	}

	for _, key := range keys {
		for rawKey, value := range raw {
			if strings.EqualFold(rawKey, key) {
				if s := rawString(value); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func oauthProviderUserID(raw map[string]interface{}) string {
	return rawFirstNonEmpty(raw, "id", "user_id", "userId", "userid", "uid", "sub", "account_id", "accountId")
}

func oauthProviderUnionID(raw map[string]interface{}) string {
	return rawFirstNonEmpty(raw, "unionid", "union_id", "unionId")
}

func oauthProviderOpenID(raw map[string]interface{}) string {
	return rawFirstNonEmpty(raw, "openid", "open_id", "openId")
}

func oauthProviderLogin(raw map[string]interface{}) string {
	return rawFirstNonEmpty(raw, "login", "username", "user_name", "preferred_username", "screen_name")
}

func oauthProviderNickname(raw map[string]interface{}) string {
	return rawFirstNonEmpty(raw, "nickname", "nick_name", "nickName")
}

func oauthProviderName(raw map[string]interface{}) string {
	return rawFirstNonEmpty(raw, "name", "display_name", "displayName", "full_name", "fullName", "real_name", "realName")
}

func oauthProviderEmail(raw map[string]interface{}) string {
	return rawFirstNonEmpty(raw, "email", "mail", "email_address", "emailAddress", "public_email", "publicEmail")
}

func oauthProviderAvatarURL(raw map[string]interface{}) string {
	return rawFirstNonEmpty(raw, "avatar_url", "avatarUrl", "avatar", "picture", "picture_url", "pictureUrl", "profile_image_url", "profileImageUrl")
}

// oauthStableID builds Id from stable provider identifiers only: userid > unionid > openid.
func oauthStableID(userid, unionid, openid, username, email string) string {
	return firstNonEmpty(userid, unionid, openid)
}

// stableIDChain returns the first non-empty string among peer id-like values in preference order.
func stableIDChain(candidates ...string) string {
	return firstNonEmpty(candidates...)
}

// oauthUsernamePreferLogin uses provider login/username only.
func oauthUsernamePreferLogin(providerLogin, userid, unionid, openid, email string) string {
	return firstNonEmpty(providerLogin)
}

// displayNameFromNickname prefers display fields first, then login.
func displayNameFromNickname(nickname, name, login, email, idFallback string) string {
	return firstNonEmpty(nickname, name, login)
}
