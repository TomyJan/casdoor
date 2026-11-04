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

import (
	"path/filepath"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/xorm-io/xorm"
	"github.com/xorm-io/xorm/names"
)

func TestMergeUserTransfersLoginIdentities(t *testing.T) {
	for _, scenario := range []struct {
		name          string
		softDelete    bool
		alreadyLinked bool
	}{
		{name: "soft deletion", softDelete: true},
		{name: "hard deletion"},
		{name: "already linked identity", softDelete: true, alreadyLinked: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			engine, err := xorm.NewEngine("sqlite", filepath.Join(t.TempDir(), "merge.db"))
			if err != nil {
				t.Fatal(err)
			}
			engine.SetMapper(names.GonicMapper{})
			previousOrmer, previousEnforcer := ormer, userEnforcer
			ormer = &Ormer{Engine: engine}
			t.Cleanup(func() {
				ormer, userEnforcer = previousOrmer, previousEnforcer
				if err := engine.Close(); err != nil {
					t.Error(err)
				}
			})
			m, err := model.NewModelFromString(`[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act`)
			if err != nil {
				t.Fatal(err)
			}
			enforcer, err := casbin.NewSyncedEnforcer(m)
			if err != nil {
				t.Fatal(err)
			}
			userEnforcer = NewUserGroupEnforcer(enforcer)
			if err := engine.Sync2(new(User), new(Organization), new(ThirdPartyLink), new(PasswordHistory), new(Token), new(Session), new(Syncer)); err != nil {
				t.Fatal(err)
			}
			org := &Organization{Owner: "admin", Name: "merge-org", EnableSoftDeletion: scenario.softDelete}
			from := &User{
				Owner: org.Name, Name: "a-source", Id: "source-id", Bilibili: "bili-openid", GitHub: "github-id",
				Email: "source@example.com", EmailVerified: true, Phone: "13800138000", CountryCode: "86",
				Balance: 12.5, Properties: map[string]string{"oauth_Bilibili_name": "Bili user", "mergedUsers": "first,a-source,z-kept"},
			}
			into := &User{
				Owner: org.Name, Name: "z-kept", Id: "kept-id", Balance: 7.25,
				Properties: map[string]string{"mergedUsers": "existing,first"},
			}
			if scenario.alreadyLinked {
				into.Bilibili, into.GitHub = from.Bilibili, from.GitHub
				into.Email, into.Phone, into.CountryCode = from.Email, from.Phone, from.CountryCode
			}
			if _, err := engine.Insert(org, from, into); err != nil {
				t.Fatal(err)
			}
			if err := MergeUser(from, into); err != nil {
				t.Fatal(err)
			}
			retained, err := getUser(org.Name, "z-kept")
			if err != nil {
				t.Fatal(err)
			}
			if retained == nil {
				t.Fatal("the retained account must remain")
			}
			if retained.Balance != 19.75 {
				t.Errorf("retained balance = %v, want 19.75", retained.Balance)
			}
			if merged := getUserProperty(retained, UserPropertiesMergedUsers); merged != "existing,first,a-source" {
				t.Errorf("merged accounts = %q, want existing,first,a-source", merged)
			}
			for _, identity := range []struct{ field, value string }{
				{"bilibili", "bili-openid"}, {"github", "github-id"}, {"email", "source@example.com"}, {"phone", "13800138000"},
			} {
				user, err := GetUserByField(org.Name, identity.field, identity.value)
				if err != nil {
					t.Fatal(err)
				}
				if user == nil {
					t.Errorf("lookup %s did not find the retained active account", identity.field)
				} else if user.Name != "z-kept" || user.Id != "kept-id" || user.IsDeleted {
					t.Errorf("lookup %s resolves to %s (%s, deleted=%v), want the retained active account", identity.field, user.Name, user.Id, user.IsDeleted)
				}
			}
			deleted, err := getUser(org.Name, "a-source")
			if err != nil {
				t.Fatal(err)
			}
			if scenario.softDelete {
				if deleted == nil || !deleted.IsDeleted {
					t.Fatal("soft deletion must preserve the deleted source row")
				}
				if deleted.Bilibili != "" || deleted.GitHub != "" || deleted.Email != "" || deleted.EmailVerified || deleted.Phone != "" || deleted.CountryCode != "" {
					t.Fatal("the deleted source must release the transferred login identities")
				}
				if deleted.Balance != 0 {
					t.Errorf("deleted source balance = %v, want 0 after transfer", deleted.Balance)
				}
			} else if deleted != nil {
				t.Fatal("hard deletion must remove the source row")
			}
		})
	}
}
