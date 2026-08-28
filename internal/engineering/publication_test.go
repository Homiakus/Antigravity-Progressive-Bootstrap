package engineering

import (
	"fmt"
	"strings"
	"testing"
)

const (
	shaBase = "1111111111111111111111111111111111111111"
	shaHead = "2222222222222222222222222222222222222222"
	shaTree = "3333333333333333333333333333333333333333"
)

func validPublicationProofText() string {
	return "branch=main;head=" + shaHead + ";remote=origin;remote-head=" + shaHead + ";base=" + shaBase + ";qualified-tree=" + shaTree + ";force=false"
}

func TestParsePublicationProof(t *testing.T) {
	proof, err := ParsePublicationProof(validPublicationProofText())
	if err != nil {
		t.Fatal(err)
	}
	if proof.Branch != "main" || proof.Head != shaHead || proof.BaseHead != shaBase || proof.QualifiedTree != shaTree || proof.Force {
		t.Fatalf("unexpected proof: %+v", proof)
	}
}

func TestPublicationProofParsingFailsClosed(t *testing.T) {
	cases := map[string]string{
		"missing-field": "branch=main;head=" + shaHead,
		"duplicate": validPublicationProofText() + ";head=" + shaHead,
		"unknown": validPublicationProofText() + ";magic=yes",
		"wrong-branch": strings.Replace(validPublicationProofText(), "branch=main", "branch=dev", 1),
		"unsafe-remote": strings.Replace(validPublicationProofText(), "remote=origin", "remote=--upload-pack=x", 1),
		"force": strings.Replace(validPublicationProofText(), "force=false", "force=true", 1),
		"bad-sha": strings.Replace(validPublicationProofText(), "head="+shaHead, "head=xyz", 1),
		"no-movement": strings.Replace(validPublicationProofText(), "base="+shaBase, "base="+shaHead, 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePublicationProof(input); err == nil {
				t.Fatalf("invalid proof %s accepted", name)
			}
		})
	}
}

func TestPublicationSnapshotMutationSentinel(t *testing.T) {
	proof, err := ParsePublicationProof(validPublicationProofText())
	if err != nil {
		t.Fatal(err)
	}
	valid := PublicationSnapshot{Root: "/repo", Branch: "main", Head: shaHead, Tree: shaTree, RemoteHead: shaHead, Clean: true, BaseAncestor: true}
	if err := ValidatePublicationSnapshot(proof, valid); err != nil {
		t.Fatal(err)
	}
	mutants := map[string]func(*PublicationSnapshot, *PublicationProof){
		"branch": func(s *PublicationSnapshot, p *PublicationProof) { s.Branch = "feature" },
		"local-head": func(s *PublicationSnapshot, p *PublicationProof) { s.Head = shaBase },
		"remote-proof": func(s *PublicationSnapshot, p *PublicationProof) { p.RemoteHead = shaBase },
		"remote-observed": func(s *PublicationSnapshot, p *PublicationProof) { s.RemoteHead = shaBase },
		"qualified-tree": func(s *PublicationSnapshot, p *PublicationProof) { s.Tree = shaBase },
		"ancestor": func(s *PublicationSnapshot, p *PublicationProof) { s.BaseAncestor = false },
		"dirty": func(s *PublicationSnapshot, p *PublicationProof) { s.Clean = false },
		"force": func(s *PublicationSnapshot, p *PublicationProof) { p.Force = true },
	}
	for name, mutate := range mutants {
		t.Run(name, func(t *testing.T) {
			snapshot := valid
			candidate := proof
			mutate(&snapshot, &candidate)
			if err := ValidatePublicationSnapshot(candidate, snapshot); err == nil {
				t.Fatalf("publication mutant %s survived", name)
			}
		})
	}
}

type fakeGitRunner struct {
	outputs map[string]string
	errors  map[string]error
}

func (f fakeGitRunner) Run(_ string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	if err := f.errors[key]; err != nil {
		return "", err
	}
	out, ok := f.outputs[key]
	if !ok {
		return "", fmt.Errorf("unexpected git call %q", key)
	}
	return out, nil
}

func TestGitPublicationVerifierUsesReadOnlyObservations(t *testing.T) {
	proof, err := ParsePublicationProof(validPublicationProofText())
	if err != nil {
		t.Fatal(err)
	}
	runner := fakeGitRunner{outputs: map[string]string{
		"rev-parse --show-toplevel": "/repo",
		"branch --show-current": "main",
		"rev-parse HEAD": shaHead,
		"rev-parse HEAD^{tree}": shaTree,
		"status --porcelain --untracked-files=normal": "",
		"ls-remote --heads origin refs/heads/main": shaHead + "\trefs/heads/main",
		"merge-base --is-ancestor " + shaBase + " " + shaHead: "",
	}}
	verifier := GitPublicationVerifier{runner: runner}
	got, err := verifier.Verify("/workspace", proof)
	if err != nil {
		t.Fatal(err)
	}
	if got.Head != shaHead || got.RemoteHead != shaHead || got.Tree != shaTree || !got.Clean || !got.BaseAncestor {
		t.Fatalf("unexpected observed state: %+v", got)
	}
}

func TestGitPublicationVerifierRejectsNonAncestor(t *testing.T) {
	proof, err := ParsePublicationProof(validPublicationProofText())
	if err != nil {
		t.Fatal(err)
	}
	key := "merge-base --is-ancestor " + shaBase + " " + shaHead
	runner := fakeGitRunner{outputs: map[string]string{
		"rev-parse --show-toplevel": "/repo",
		"branch --show-current": "main",
		"rev-parse HEAD": shaHead,
		"rev-parse HEAD^{tree}": shaTree,
		"status --porcelain --untracked-files=normal": "",
		"ls-remote --heads origin refs/heads/main": shaHead + "\trefs/heads/main",
	}, errors: map[string]error{key: fmt.Errorf("exit status 1")}}
	verifier := GitPublicationVerifier{runner: runner}
	if _, err := verifier.Verify("/workspace", proof); err == nil || !strings.Contains(err.Error(), "fast-forward") {
		t.Fatalf("expected fast-forward rejection, got %v", err)
	}
}
