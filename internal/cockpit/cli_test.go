package cockpit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	stdout []byte
	stderr []byte
	err    error
	args   []string
}

func (f *fakeRunner) Run(_ context.Context, binary string, args ...string) ([]byte, []byte, error) {
	f.args = append([]string{binary}, args...)
	return f.stdout, f.stderr, f.err
}

func TestCLIRejectsProtocolMismatch(t *testing.T) {
	runner := &fakeRunner{stdout: []byte(`{"protocolVersion":2,"ok":true,"data":{"protocolVersion":2}}`)}
	client, _ := NewCLI("cockpit-control-test", runner)
	if _, err := client.Protocol(context.Background()); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected protocol mismatch, got %v", err)
	}
}

func TestCLIListAccountsUsesSanitizedContract(t *testing.T) {
	runner := &fakeRunner{stdout: []byte(`{"protocolVersion":1,"ok":true,"data":[{"id":"a1","email":"a@example.com","plan":"pro"}]}`)}
	client, _ := NewCLI("cockpit-control-test", runner)
	accounts, err := client.ListAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].ID != "a1" {
		t.Fatalf("accounts=%#v", accounts)
	}
	expected := []string{"cockpit-control-test", "accounts", "list", "--json"}
	if !reflect.DeepEqual(runner.args, expected) {
		t.Fatalf("args=%v want=%v", runner.args, expected)
	}
}

func TestCLICreateInstanceCarriesWorkingDirectory(t *testing.T) {
	runner := &fakeRunner{stdout: []byte(`{"protocolVersion":1,"ok":true,"data":{"id":"i1","userDataDir":"C:/profile","workingDir":"C:/repo","running":false,"initialized":true}}`)}
	client, _ := NewCLI("cockpit-control-test", runner)
	instance, err := client.CreateInstance(context.Background(), CreateInstanceSpec{Name: "repo", UserDataDir: "C:/profile", WorkingDir: "C:/repo", BindAccountID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if instance.WorkingDir != "C:/repo" {
		t.Fatalf("workingDir=%q", instance.WorkingDir)
	}
	joined := strings.Join(runner.args, " ")
	for _, want := range []string{"--working-dir C:/repo", "--account-id a1", "--json"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args %q missing %q", joined, want)
		}
	}
}

func TestCLIReportsProcessStderrWithoutParsingItAsJSON(t *testing.T) {
	runner := &fakeRunner{stderr: []byte("instance missing"), err: errors.New("exit status 1")}
	client, _ := NewCLI("cockpit-control-test", runner)
	if _, err := client.ListInstances(context.Background()); err == nil || !strings.Contains(err.Error(), "instance missing") {
		t.Fatalf("err=%v", err)
	}
}

func TestCLIRejectsSecretFieldsFromControlBinary(t *testing.T) {
	runner := &fakeRunner{stdout: []byte(`{"protocolVersion":1,"ok":true,"data":[{"id":"a1","email":"a@example.com","refreshToken":"secret"}]}`)}
	client, _ := NewCLI("cockpit-control-test", runner)
	if _, err := client.ListAccounts(context.Background()); err == nil {
		t.Fatal("expected unknown secret field to be rejected")
	}
}
