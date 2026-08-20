package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/assets"
	"github.com/homiakus/agctl/internal/execx"
	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

type Pack struct {
	ID           string
	Label        string
	Repo         string
	SkillsSubdir string
	Recommended  bool
}

var Packs = []Pack{
	{ID: "superpowers", Label: "Superpowers", Repo: "https://github.com/obra/superpowers.git", SkillsSubdir: "skills", Recommended: true},
	{ID: "agent-skills", Label: "Addy Osmani Agent Skills", Repo: "https://github.com/addyosmani/agent-skills.git", SkillsSubdir: "skills", Recommended: true},
	{ID: "gemini-skills", Label: "Google Gemini Skills", Repo: "https://github.com/google-gemini/gemini-skills.git", SkillsSubdir: "skills", Recommended: false},
	{ID: "no-ai-slop", Label: "No AI Slop", Repo: "https://github.com/petergyang/no-ai-slop.git", SkillsSubdir: "skills", Recommended: true},
}

type Item struct {
	Name   string
	Source string
	Path   string
}

func InstallEmbedded(p paths.Paths) error {
	for _, name := range assets.SkillNames() {
		if err := assets.InstallSkill(name, p.GlobalSkillsRoot); err != nil {
			return err
		}
		// CLI global skills are flat markdown files; copy the main entry there too.
		b, err := assets.ReadSkillFile(name, "SKILL.md")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(p.CLISkillsRoot, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(p.CLISkillsRoot, name+".md"), cliSkillContent(b, filepath.Join(p.GlobalSkillsRoot, name)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func SyncPack(p paths.Paths, id string) ([]Item, error) {
	pack, ok := findPack(id)
	if !ok {
		return nil, fmt.Errorf("unknown skill pack %q", id)
	}
	if !execx.Exists("git") {
		return nil, fmt.Errorf("git is required to sync %s", pack.Label)
	}

	cache := filepath.Join(p.AppRoot, "cache", "skill-packs", pack.ID)
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(cache, ".git")); err == nil {
		if _, err := execx.Run(2*time.Minute, cache, "git", "fetch", "--depth=1", "origin"); err != nil {
			return nil, err
		}
		if _, err := execx.Run(2*time.Minute, cache, "git", "reset", "--hard", "origin/HEAD"); err != nil {
			// origin/HEAD is not always configured in shallow clones; fall back to pull.
			if _, err2 := execx.Run(2*time.Minute, cache, "git", "pull", "--ff-only"); err2 != nil {
				return nil, err2
			}
		}
	} else {
		_ = os.RemoveAll(cache)
		if _, err := execx.Run(5*time.Minute, "", "git", "clone", "--depth=1", pack.Repo, cache); err != nil {
			return nil, err
		}
	}

	root := filepath.Join(cache, filepath.FromSlash(pack.SkillsSubdir))
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read pack skills %s: %w", root, err)
	}
	sharedRefs := filepath.Join(cache, "references")
	var installed []Item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		src := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(src, "SKILL.md")); err != nil {
			continue
		}
		dst := filepath.Join(p.GlobalSkillsRoot, e.Name())
		if err := copyTreeReplace(src, dst); err != nil {
			return installed, err
		}
		// Some packs keep references at repository root while SKILL.md uses references/foo.md.
		// Mirror missing shared references into the skill so links stay self-contained.
		if st, err := os.Stat(sharedRefs); err == nil && st.IsDir() {
			_ = copyTreeMerge(sharedRefs, filepath.Join(dst, "references"))
		}
		// CLI global skills are flat Markdown. Preserve access to bundled resources
		// by adding an explicit absolute resource base that points at the IDE/global
		// skill directory, where scripts/references/assets are copied in full.
		b, _ := os.ReadFile(filepath.Join(dst, "SKILL.md"))
		if err := os.MkdirAll(p.CLISkillsRoot, 0o755); err != nil {
			return installed, err
		}
		if err := os.WriteFile(filepath.Join(p.CLISkillsRoot, e.Name()+".md"), cliSkillContent(b, dst), 0o644); err != nil {
			return installed, err
		}
		installed = append(installed, Item{Name: e.Name(), Source: pack.ID, Path: dst})
	}
	if len(installed) == 0 {
		return nil, fmt.Errorf("no SKILL.md directories found in %s", pack.Repo)
	}
	sort.Slice(installed, func(i, j int) bool { return installed[i].Name < installed[j].Name })
	commit, _ := execx.Run(30*time.Second, cache, "git", "rev-parse", "HEAD")
	commit = strings.TrimSpace(commit)
	_ = writePackLock(p, pack, commit, installed)
	return installed, nil
}

func writePackLock(p paths.Paths, pack Pack, commit string, installed []Item) error {
	files := map[string]string{}
	for _, item := range installed {
		err := filepath.Walk(item.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info == nil || !info.Mode().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(item.Path, path)
			if err != nil {
				return err
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(b)
			key := filepath.ToSlash(filepath.Join(item.Name, rel))
			files[key] = hex.EncodeToString(sum[:])
			return nil
		})
		if err != nil {
			return err
		}
	}

	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		_, _ = h.Write([]byte(k + "=" + files[k] + "\n"))
	}

	lock := model.ProvenanceLock{
		ID: pack.ID, Kind: "skill-pack", Source: pack.Repo, Commit: commit,
		SHA256: hex.EncodeToString(h.Sum(nil)), InstalledAt: time.Now().UTC().Format(time.RFC3339Nano), Path: p.GlobalSkillsRoot, Files: files,
	}
	return jsonx.WriteAtomic(filepath.Join(p.LocksRoot, "skill-pack-"+pack.ID+".json"), lock, p.BackupsRoot)
}

func cliSkillContent(content []byte, resourceBase string) []byte {
	base := filepath.Clean(resourceBase)
	note := fmt.Sprintf(`

## agctl CLI resource base

This flat CLI skill is mirrored from the full skill directory at "%s".
When these instructions refer to relative resources such as references/, scripts/,
examples/, resources/, or assets/, resolve those paths relative to that directory.
`, base)
	out := append([]byte(nil), content...)
	out = append(out, []byte(note)...)
	return out
}

func SyncRecommended(p paths.Paths) (map[string]int, error) {
	if err := InstallEmbedded(p); err != nil {
		return nil, err
	}
	result := map[string]int{"embedded": len(assets.SkillNames())}
	for _, pack := range Packs {
		if !pack.Recommended {
			continue
		}
		items, err := SyncPack(p, pack.ID)
		if err != nil {
			// A remote pack failure should not erase already-installed local capabilities.
			result[pack.ID] = -1
			continue
		}
		result[pack.ID] = len(items)
	}
	return result, nil
}

func List(p paths.Paths) ([]Item, error) {
	entries, err := os.ReadDir(p.GlobalSkillsRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skill := filepath.Join(p.GlobalSkillsRoot, e.Name(), "SKILL.md")
		if _, err := os.Stat(skill); err == nil {
			out = append(out, Item{Name: e.Name(), Path: skill})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func Remove(p paths.Paths, name string) error {
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\\`) || strings.TrimSpace(name) == "" {
		return fmt.Errorf("invalid skill name")
	}
	if err := os.RemoveAll(filepath.Join(p.GlobalSkillsRoot, name)); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(p.CLISkillsRoot, name+".md"))
	return nil
}

func findPack(id string) (Pack, bool) {
	for _, p := range Packs {
		if p.ID == id {
			return p, true
		}
	}
	return Pack{}, false
}

func copyTreeReplace(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return copyTreeMerge(src, dst)
}

func copyTreeMerge(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if _, err := os.Stat(target); err == nil {
			// Merge mode intentionally keeps a skill-local reference over a shared one.
			if src != path && strings.Contains(filepath.ToSlash(src), "/references") {
				return nil
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			in.Close()
			return err
		}
		_, cpErr := io.Copy(out, in)
		inCloseErr := in.Close()
		outCloseErr := out.Close()
		if cpErr != nil {
			return cpErr
		}
		if inCloseErr != nil {
			return inCloseErr
		}
		return outCloseErr
	})
}
