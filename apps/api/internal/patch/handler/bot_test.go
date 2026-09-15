package handler

import "testing"

func TestValidateBotResource(t *testing.T) {
	ok := []string{"zh-Hans"}
	win := []string{"windows"}
	if msg := validateBotResource(nil, ok, win); msg == "" {
		t.Fatal("empty types")
	}
	if msg := validateBotResource([]string{"manual"}, nil, win); msg == "" {
		t.Fatal("empty language")
	}
	if msg := validateBotResource([]string{"manual"}, ok, nil); msg == "" {
		t.Fatal("empty platform")
	}
	if msg := validateBotResource([]string{"crack"}, ok, win); msg == "" {
		t.Fatal("crack must be refused")
	}
	if msg := validateBotResource([]string{"nope"}, ok, win); msg == "" {
		t.Fatal("unknown type")
	}
	if msg := validateBotResource([]string{"manual"}, []string{"jp"}, win); msg == "" {
		t.Fatal("unknown language")
	}
	if msg := validateBotResource([]string{"manual"}, ok, ok); msg == "" {
		t.Fatal("unknown platform")
	}
	if msg := validateBotResource([]string{"manual"}, ok, win); msg != "" {
		t.Fatalf("manual: %s", msg)
	}
	if msg := validateBotResource([]string{"fix"}, []string{"ja"}, win); msg != "" {
		t.Fatalf("fix: %s", msg)
	}
}
