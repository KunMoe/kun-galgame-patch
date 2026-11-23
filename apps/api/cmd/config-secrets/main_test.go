package main

import (
	"reflect"
	"strings"
	"testing"

	"kun-galgame-patch-api/pkg/config"
)

func TestPlanStoresSecretsAndNamedKeysOnly(t *testing.T) {
	env := []string{
		"PATH=/usr/bin",
		config.DatabaseURLEnv + "=postgresql://postgres:x@postgres:5432/kungalgame_patch",
		"KUN_CONFIG_KEK=k",
		"REDIS_HOST=redis",
		"OAUTH_CLIENT_ID=df3f",
		"OAUTH_CLIENT_SECRET=s",
		"KUN_STORE_API_KEY=nmk",
		"KUN_TRUST_CALLBACK_SECRET=",
		"KUN_BOT_SUBMIT_USER_ID=125528",
		"KUN_VISUAL_NOVEL_S3_STORAGE_SECRET_ACCESS_KEY=ghost",
	}
	read := map[string]bool{}
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		read[name] = true
	}
	delete(read, "KUN_VISUAL_NOVEL_S3_STORAGE_SECRET_ACCESS_KEY")
	got := Plan(env, "KUN_BOT_SUBMIT_USER_ID, REDIS_HOST", read)
	want := []pair{
		{"KUN_BOT_SUBMIT_USER_ID", "125528"},
		{"KUN_STORE_API_KEY", "nmk"},
		{"OAUTH_CLIENT_SECRET", "s"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %v, want %v", got, want)
	}
}
