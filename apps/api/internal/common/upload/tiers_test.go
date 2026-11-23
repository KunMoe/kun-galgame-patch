package upload

import (
	"testing"

	"kun-galgame-patch-api/internal/constants"
)

type fakeSettings map[string]string

func (f fakeSettings) GetString(key string) string { return f[key] }
func (f fakeSettings) SetString(key, value string, _ int) error {
	f[key] = value
	return nil
}

func TestTierConvertsGiBAndUnlimited(t *testing.T) {
	got := TierSetting{MaxFileGiB: 10, DailyGiB: 100}.tier()
	if got.MaxFileSize != 10*oneGiBBytes || got.DailyLimit != 100*oneGiBBytes {
		t.Fatalf("tier = %+v", got)
	}
	if got := (TierSetting{MaxFileGiB: 20}).tier(); got.DailyLimit != constants.UnlimitedDailyUpload {
		t.Fatalf("daily 0 must be unlimited, got %d", got.DailyLimit)
	}
}

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		why string
		t   TierSetting
		ok  bool
	}{
		{"defaults", DefaultTiers.User, true},
		{"unlimited daily", TierSetting{MaxFileGiB: 20, DailyGiB: 0}, true},
		{"file above artifact's cap", TierSetting{MaxFileGiB: MaxFileCapGiB + 1, DailyGiB: 100}, false},
		{"zero file size", TierSetting{MaxFileGiB: 0, DailyGiB: 100}, false},
		{"daily below one file", TierSetting{MaxFileGiB: 10, DailyGiB: 5}, false},
	} {
		if err := tc.t.validate("x"); (err == nil) != tc.ok {
			t.Errorf("%s: err = %v", tc.why, err)
		}
	}
}

func TestLoadFallsBackPerRoleAndRoundTrips(t *testing.T) {
	s := &TierStore{settings: fakeSettings{}}
	if got := s.Load(); got != DefaultTiers {
		t.Fatalf("empty setting = %+v, want defaults", got)
	}

	s.settings = fakeSettings{tiersSettingKey: `{"user":{"max_file_gib":5,"daily_gib":50},"creator":{"max_file_gib":99,"daily_gib":1}}`}
	got := s.Load()
	if got.User != (TierSetting{5, 50}) || got.Creator != DefaultTiers.Creator || got.Admin != DefaultTiers.Admin {
		t.Fatalf("partial / invalid setting = %+v", got)
	}

	s.settings = fakeSettings{tiersSettingKey: `not json`}
	if got := s.Load(); got != DefaultTiers {
		t.Fatalf("broken setting = %+v, want defaults", got)
	}

	want := DefaultTiers
	want.User = TierSetting{MaxFileGiB: 8, DailyGiB: 80}
	if err := s.Save(want, 1); err != nil {
		t.Fatal(err)
	}
	if got := s.Load(); got != want {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
	bad := DefaultTiers
	bad.Admin.MaxFileGiB = 0
	if err := s.Save(bad, 1); err == nil {
		t.Fatal("an invalid tier was saved")
	}
}
