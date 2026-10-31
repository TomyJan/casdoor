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

	"github.com/casdoor/casdoor/conf"
	"github.com/xorm-io/xorm"
	"github.com/xorm-io/xorm/names"
)

func setupSettingTest(t *testing.T) *xorm.Engine {
	t.Helper()
	engine, err := xorm.NewEngine("sqlite", filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	engine.SetMapper(names.GonicMapper{})
	previousOrmer := ormer
	wasVisible := conf.GetWebConfig().ShowCloudPromotion
	ormer = &Ormer{Engine: engine}
	t.Cleanup(func() {
		ormer = previousOrmer
		conf.SetHideCloudPromotion(!wasVisible)
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	return engine
}

func TestCloudPromotionHiddenForNewInstallation(t *testing.T) {
	engine := setupSettingTest(t)
	if err := engine.Sync2(new(Setting)); err != nil {
		t.Fatal(err)
	}
	setting := initBuiltInSetting()
	if !setting.HideCloudPromotion {
		t.Fatal("a new installation must hide cloud promotion by default")
	}
	setting.apply()
	if conf.GetWebConfig().ShowCloudPromotion {
		t.Fatal("the default database setting must hide frontend promotion")
	}

	for _, hide := range []bool{false, true} {
		setting.HideCloudPromotion = hide
		updated, err := UpdateSetting(setting)
		if err != nil || !updated {
			t.Fatalf("UpdateSetting() = (%v, %v)", updated, err)
		}
		saved, err := GetSetting()
		if err != nil {
			t.Fatal(err)
		}
		if saved == nil || saved.HideCloudPromotion != hide {
			t.Fatalf("saved setting = %+v, want HideCloudPromotion %v", saved, hide)
		}
		if got := conf.GetWebConfig().ShowCloudPromotion; got != !hide {
			t.Fatalf("ShowCloudPromotion = %v after saving hide %v", got, hide)
		}
	}
}

func TestCloudPromotionHiddenWhenUpgradingSettings(t *testing.T) {
	engine := setupSettingTest(t)
	// Reproduce an existing settings row created before promotion controls existed.
	if _, err := engine.Exec("CREATE TABLE setting (owner TEXT NOT NULL, name TEXT NOT NULL, PRIMARY KEY (owner, name))"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exec("INSERT INTO setting (owner, name) VALUES (?, ?)", "admin", "setting-built-in"); err != nil {
		t.Fatal(err)
	}
	if err := engine.Sync2(new(Setting)); err != nil {
		t.Fatal(err)
	}
	setting := initBuiltInSetting()
	if !setting.HideCloudPromotion {
		t.Fatal("an existing installation must get the hidden default when its schema is upgraded")
	}
	setting.apply()
	if conf.GetWebConfig().ShowCloudPromotion {
		t.Fatal("upgrading an existing installation must not enable frontend promotion")
	}
}
