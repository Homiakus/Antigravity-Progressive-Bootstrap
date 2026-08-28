package main

import (
	"strings"
	"testing"

	"github.com/homiakus/agctl/internal/remote/model"
)

func TestParseBootstrapOpen(t *testing.T) {
	const repositoryID = "rep_1700000000000_00000000000000000001"
	got, err := parseBootstrapOpen(repositoryID + ":account-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.RepositoryID != model.RepositoryID(repositoryID) || got.AccountID != "account-1" {
		t.Fatalf("bootstrap=%#v", got)
	}
}

func TestParseBootstrapOpenRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "bad", "bad:account", "rep_1700000000000_00000000000000000001:"} {
		if _, err := parseBootstrapOpen(input); err == nil {
			t.Fatalf("expected %q to fail", input)
		}
	}
}

func TestTelegramBotKeyIsStableFingerprintNotToken(t *testing.T) {
	const token = "123456:SUPER-SECRET-BOT-TOKEN"
	first := telegramBotKey(token)
	second := telegramBotKey(token)
	if first != second {
		t.Fatalf("unstable bot key %q != %q", first, second)
	}
	if !strings.HasPrefix(first, "telegram:") || strings.Contains(first, token) || strings.Contains(first, "SUPER-SECRET") {
		t.Fatalf("unsafe bot key %q", first)
	}
	if len(first) != len("telegram:")+16 {
		t.Fatalf("unexpected fingerprint length %d", len(first))
	}
}
