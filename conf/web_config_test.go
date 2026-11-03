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

package conf

import "testing"

func TestCloudPromotionHiddenBeforeSettingsLoad(t *testing.T) {
	if GetWebConfig().ShowCloudPromotion {
		t.Fatal("cloud promotion must stay hidden before database settings are loaded")
	}
}

func TestCloudPromotionRespectsExplicitSetting(t *testing.T) {
	wasVisible := GetWebConfig().ShowCloudPromotion
	t.Cleanup(func() { SetHideCloudPromotion(!wasVisible) })

	for _, hide := range []bool{true, false, true} {
		SetHideCloudPromotion(hide)
		if got := GetWebConfig().ShowCloudPromotion; got != !hide {
			t.Fatalf("ShowCloudPromotion = %v after setting hide to %v", got, hide)
		}
	}
}
