package engineering

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var (
	gitObjectRE = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
	remoteNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// PublicationProof is the coordinator's structured claim about the already
// completed publication. It is not authorization to push and it never performs
// a mutation itself.
type PublicationProof struct {
	Branch        string
	Head          string
	Remote        string
	RemoteHead    string
	BaseHead      string
	QualifiedTree string
	Force         bool
}

// PublicationSnapshot is independently observed repository state used to
// distinguish a real fast-forward publication from free-form completion text.
type PublicationSnapshot struct {
	Root         string
	Branch       string
	Head         string
	Tree         string
	RemoteHead   string
	Clean        bool
	BaseAncestor bool
}

// PublicationVerifier verifies an already-published repository state. The
// interface makes the completion boundary testable without network/git process
// dependence.
type PublicationVerifier interface {
	Verify(workspace string, proof PublicationProof) (PublicationSnapshot, error)
}

// ParsePublicationProof parses the push-main evidence value. Unknown,
// duplicate, or missing fields fail closed.
func ParsePublicationProof(value string) (PublicationProof, error) {
	fields := map[string]string{}
	for _, raw := range strings.Split(value, ";") {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			return PublicationProof{}, fmt.Errorf("invalid push-main proof field %q", part)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		if key == "" || val == "" {
			return PublicationProof{}, fmt.Errorf("empty push-main proof field %q", part)
		}
		if _, exists := fields[key]; exists {
			return PublicationProof{}, fmt.Errorf("duplicate push-main proof field %q", key)
		}
		fields[key] = val
	}
	allowed := map[string]bool{
		"branch": true, "head": true, "remote": true, "remote-head": true,
		"base": true, "qualified-tree": true, "force": true,
	}
	for key := range fields {
		if !allowed[key] {
			return PublicationProof{}, fmt.Errorf("unknown push-main proof field %q", key)
		}
	}
	for _, key := range []string{"branch", "head", "remote", "remote-head", "base", "qualified-tree", "force"} {
		if fields[key] == "" {
			return PublicationProof{}, fmt.Errorf("push-main proof missing %q", key)
		}
	}
	force, err := strconv.ParseBool(fields["force"])
	if err != nil {
		return PublicationProof{}, fmt.Errorf("invalid push-main force flag: %w", err)
	}
	proof := PublicationProof{
		Branch: fields["branch"], Head: strings.ToLower(fields["head"]),
		Remote: fields["remote"], RemoteHead: strings.ToLower(fields["remote-head"]),
		BaseHead: strings.ToLower(fields["base"]), QualifiedTree: strings.ToLower(fields["qualified-tree"]),
		Force: force,
	}
	if proof.Branch != "main" {
		return PublicationProof{}, fmt.Errorf("managed autonomous publication must target main, got %q", proof.Branch)
	}
	if !remoteNameRE.MatchString(proof.Remote) {
		return PublicationProof{}, fmt.Errorf("invalid git remote name %q", proof.Remote)
	}
	for label, value := range map[string]string{
		"head": proof.Head, "remote-head": proof.RemoteHead, "base": proof.BaseHead, "qualified-tree": proof.QualifiedTree,
	} {
		if !gitObjectRE.MatchString(value) {
			return PublicationProof{}, fmt.Errorf("invalid %s object id %q", label, value)
		}
	}
	if proof.Force {
		return PublicationProof{}, fmt.Errorf("force publication is forbidden")
	}
	if proof.BaseHead == proof.Head {
		return PublicationProof{}, fmt.Errorf("publication base and head must differ")
	}
	return proof, nil
}

// ValidatePublicationSnapshot applies the mutation-resistant semantic checks to
// an observed repository snapshot.
func ValidatePublicationSnapshot(proof PublicationProof, observed PublicationSnapshot) error {
	if proof.Force {
		return fmt.Errorf("force publication is forbidden")
	}
	if observed.Branch != proof.Branch {
		return fmt.Errorf("publication branch mismatch: observed=%q proof=%q", observed.Branch, proof.Branch)
	}
	if observed.Head != proof.Head {
		return fmt.Errorf("local HEAD mismatch: observed=%s proof=%s", observed.Head, proof.Head)
	}
	if observed.RemoteHead != proof.RemoteHead {
		return fmt.Errorf("remote HEAD mismatch: observed=%s proof=%s", observed.RemoteHead, proof.RemoteHead)
	}
	if observed.Head != observed.RemoteHead {
		return fmt.Errorf("main is not synchronized: local=%s remote=%s", observed.Head, observed.RemoteHead)
	}
	if observed.Tree != proof.QualifiedTree {
		return fmt.Errorf("published tree differs from qualified tree: observed=%s qualified=%s", observed.Tree, proof.QualifiedTree)
	}
	if !observed.BaseAncestor {
		return fmt.Errorf("publication is not a proven fast-forward from base %s", proof.BaseHead)
	}
	if !observed.Clean {
		return fmt.Errorf("repository has uncommitted or staged changes after publication")
	}
	return nil
}

type gitCommandRunner interface {
	Run(dir string, args ...string) (string, error)
}

type realGitRunner struct{}

func (realGitRunner) Run(dir string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", cmdArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// GitPublicationVerifier observes the repository using read-only git commands.
// It never fetches, commits, merges, updates refs, or pushes.
type GitPublicationVerifier struct {
	runner gitCommandRunner
}

func NewGitPublicationVerifier() GitPublicationVerifier {
	return GitPublicationVerifier{runner: realGitRunner{}}
}

func (v GitPublicationVerifier) Verify(workspace string, proof PublicationProof) (PublicationSnapshot, error) {
	if v.runner == nil {
		v.runner = realGitRunner{}
	}
	root, err := v.runner.Run(workspace, "rev-parse", "--show-toplevel")
	if err != nil {
		return PublicationSnapshot{}, err
	}
	branch, err := v.runner.Run(root, "branch", "--show-current")
	if err != nil {
		return PublicationSnapshot{}, err
	}
	head, err := v.runner.Run(root, "rev-parse", "HEAD")
	if err != nil {
		return PublicationSnapshot{}, err
	}
	tree, err := v.runner.Run(root, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return PublicationSnapshot{}, err
	}
	status, err := v.runner.Run(root, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return PublicationSnapshot{}, err
	}
	remoteRaw, err := v.runner.Run(root, "ls-remote", "--heads", proof.Remote, "refs/heads/"+proof.Branch)
	if err != nil {
		return PublicationSnapshot{}, err
	}
	remoteFields := strings.Fields(remoteRaw)
	if len(remoteFields) != 2 || remoteFields[0] == "" {
		return PublicationSnapshot{}, fmt.Errorf("remote %s branch %s did not resolve exactly one head", proof.Remote, proof.Branch)
	}
	remoteHead := strings.ToLower(remoteFields[0])
	if !gitObjectRE.MatchString(remoteHead) {
		return PublicationSnapshot{}, fmt.Errorf("remote returned invalid head %q", remoteHead)
	}
	_, ancestorErr := v.runner.Run(root, "merge-base", "--is-ancestor", proof.BaseHead, head)
	observed := PublicationSnapshot{
		Root: root, Branch: strings.TrimSpace(branch), Head: strings.ToLower(strings.TrimSpace(head)),
		Tree: strings.ToLower(strings.TrimSpace(tree)), RemoteHead: remoteHead,
		Clean: strings.TrimSpace(status) == "", BaseAncestor: ancestorErr == nil,
	}
	if err := ValidatePublicationSnapshot(proof, observed); err != nil {
		return observed, err
	}
	return observed, nil
}
