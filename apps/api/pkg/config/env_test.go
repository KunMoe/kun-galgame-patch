package config

import "testing"

func TestLookupPrefersEnvironmentThenStoreThenDefault(t *testing.T) {
	mu.Lock()
	saved := store
	store = storeState{values: map[string]string{"KUN_T_STORED": "from-store", "KUN_T_BOTH": "from-store"}}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		store = saved
		mu.Unlock()
	})
	t.Setenv("KUN_T_BOTH", "from-env")

	for key, want := range map[string]string{
		"KUN_T_BOTH":    "from-env",
		"KUN_T_STORED":  "from-store",
		"KUN_T_NEITHER": "fallback",
	} {
		if got := lookup(key, "fallback"); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestUnreadEnvNamesOnlyOwnVariablesNothingRead(t *testing.T) {
	t.Setenv("KUN_T_GHOST", "x")
	t.Setenv("KUN_T_READ", "x")
	t.Setenv("HOME_T_FOREIGN", "x")
	lookup("KUN_T_READ", "")

	unread := map[string]bool{}
	for _, name := range UnreadEnv() {
		unread[name] = true
	}
	if !unread["KUN_T_GHOST"] || unread["KUN_T_READ"] || unread["HOME_T_FOREIGN"] {
		t.Fatalf("unread = %v", UnreadEnv())
	}
}

func TestStorableKeepsWiringInTheComposeFile(t *testing.T) {
	for key, want := range map[string]bool{
		"OAUTH_CLIENT_SECRET": true,
		"KUN_NEXTMOE_API_KEY": true,
		KEKEnv:                false,
		DatabaseURLEnv:        false,
		"REDIS_HOST":          false,
		"KUN_SERVER_PORT":     false,
		"KUN_SERVER_MODE":     false,
		"PATH":                false,
	} {
		if got := Storable(key); got != want {
			t.Errorf("Storable(%s) = %v, want %v", key, got, want)
		}
	}
}
