package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/execx"
	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

type Item struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Scope      string   `json:"scope"`
	Components []string `json:"components"`
	Valid      bool     `json:"valid"`
	Issues     []string `json:"issues,omitempty"`
}

func Root(p paths.Paths, workspace string) string {
	if strings.TrimSpace(workspace) == "" {
		return p.GlobalPluginsRoot
	}
	return paths.WorkspacePlugins(workspace)
}

func List(p paths.Paths, workspace string) ([]Item, error) {
	if strings.TrimSpace(workspace) != "" {
		return listRoot(Root(p, workspace), "workspace")
	}

	ide, err := listRoot(p.GlobalPluginsRoot, "global")
	if err != nil {
		return nil, err
	}
	cli, err := listRoot(p.CLIPluginsRoot, "cli-global")
	if err != nil {
		return nil, err
	}
	byName := map[string]Item{}
	for _, it := range ide {
		byName[it.Name] = it
	}
	for _, it := range cli {
		if cur, ok := byName[it.Name]; ok {
			cur.Scope = "global+cli"
			cur.Components = unionStrings(cur.Components, it.Components)
			cur.Valid = cur.Valid && it.Valid
			for _, issue := range it.Issues {
				cur.Issues = append(cur.Issues, "CLI: "+issue)
			}
			byName[it.Name] = cur
		} else {
			byName[it.Name] = it
		}
	}
	out := make([]Item, 0, len(byName))
	for _, it := range byName {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func listRoot(root, sc string) ([]Item, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
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
		out = append(out, inspect(filepath.Join(root, e.Name()), sc))
	}
	return out, nil
}

func Inspect(path string) Item { return inspect(path, "local") }

func inspect(dir, sc string) Item {
	item := Item{Name: filepath.Base(dir), Path: dir, Scope: sc, Valid: true}
	mf := filepath.Join(dir, "plugin.json")
	manifest, err := jsonx.Read(mf, model.PluginManifest{})
	if err != nil {
		item.Valid = false
		item.Issues = append(item.Issues, "plugin.json: "+err.Error())
	} else if _, e := os.Stat(mf); e != nil {
		item.Valid = false
		item.Issues = append(item.Issues, "missing plugin.json")
	} else if manifest.Name != "" {
		item.Name = manifest.Name
	}
	checks := []struct{ name, path string }{{"skills", "skills"}, {"agents", "agents"}, {"rules", "rules"}, {"sidecars", "sidecars"}, {"mcp", "mcp_config.json"}, {"hooks", "hooks.json"}}
	for _, c := range checks {
		if st, e := os.Stat(filepath.Join(dir, c.path)); e == nil && (st.IsDir() || !st.IsDir()) {
			item.Components = append(item.Components, c.name)
		}
	}
	if _, e := os.Stat(filepath.Join(dir, "mcp_config.json")); e == nil {
		if _, er := jsonx.ReadMap(filepath.Join(dir, "mcp_config.json")); er != nil {
			item.Valid = false
			item.Issues = append(item.Issues, "invalid mcp_config.json: "+er.Error())
		}
	}
	if _, e := os.Stat(filepath.Join(dir, "hooks.json")); e == nil {
		if _, er := jsonx.ReadMap(filepath.Join(dir, "hooks.json")); er != nil {
			item.Valid = false
			item.Issues = append(item.Issues, "invalid hooks.json: "+er.Error())
		}
	}
	sort.Strings(item.Components)
	return item
}

func InstallDir(p paths.Paths, workspace, source string) (Item, error) {
	abs, err := filepath.Abs(source)
	if err != nil {
		return Item{}, err
	}
	it := inspect(abs, "source")
	if !it.Valid {
		return it, fmt.Errorf("invalid plugin: %s", strings.Join(it.Issues, "; "))
	}
	name := safeName(it.Name)
	if name == "" {
		return it, fmt.Errorf("invalid plugin name %q", it.Name)
	}
	dst := filepath.Join(Root(p, workspace), name)
	if err := copyTreeReplace(abs, dst); err != nil {
		return Item{}, err
	}
	if strings.TrimSpace(workspace) == "" {
		if err := mirrorToCLI(p, dst, name); err != nil {
			return Item{}, fmt.Errorf("mirror plugin to Antigravity CLI: %w", err)
		}
	}
	installed := inspect(dst, scope(workspace))
	_ = writeLock(p, name, "plugin", abs, "", "", dst)
	return installed, nil
}

func InstallGit(p paths.Paths, workspace, repo, ref string) (Item, error) {
	if !execx.Exists("git") {
		return Item{}, fmt.Errorf("git is required")
	}
	cache := filepath.Join(p.AppRoot, "cache", "plugins", shortHash(repo+"@"+ref))
	_ = os.RemoveAll(cache)
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		return Item{}, err
	}
	args := []string{"clone", "--depth=1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, repo, cache)
	if _, err := execx.Run(5*time.Minute, "", "git", args...); err != nil {
		return Item{}, err
	}
	commit, _ := execx.Run(30*time.Second, cache, "git", "rev-parse", "HEAD")
	commit = strings.TrimSpace(commit)
	it := inspect(cache, "source")
	if !it.Valid {
		return it, fmt.Errorf("repository root is not an Antigravity plugin: %s", strings.Join(it.Issues, "; "))
	}
	name := safeName(it.Name)
	dst := filepath.Join(Root(p, workspace), name)
	if err := copyTreeReplace(cache, dst); err != nil {
		return Item{}, err
	}
	if strings.TrimSpace(workspace) == "" {
		if err := mirrorToCLI(p, dst, name); err != nil {
			return Item{}, fmt.Errorf("mirror plugin to Antigravity CLI: %w", err)
		}
	}
	installed := inspect(dst, scope(workspace))
	_ = writeLock(p, name, "plugin", repo, ref, commit, dst)
	return installed, nil
}

// ImportBundle synthesizes a standards-compliant Antigravity plugin from a repository
// that has compatible skills/rules/hooks/MCP components but no plugin.json.
func ImportBundle(p paths.Paths, workspace, repo, ref, name string) (Item, error) {
	if !execx.Exists("git") {
		return Item{}, fmt.Errorf("git is required")
	}
	if safeName(name) == "" {
		return Item{}, fmt.Errorf("valid --name required")
	}
	cache := filepath.Join(p.AppRoot, "cache", "bundles", shortHash(repo+"@"+ref))
	_ = os.RemoveAll(cache)
	_ = os.MkdirAll(filepath.Dir(cache), 0o755)
	args := []string{"clone", "--depth=1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, repo, cache)
	if _, err := execx.Run(5*time.Minute, "", "git", args...); err != nil {
		return Item{}, err
	}
	commit, _ := execx.Run(30*time.Second, cache, "git", "rev-parse", "HEAD")
	commit = strings.TrimSpace(commit)
	dst := filepath.Join(Root(p, workspace), safeName(name))
	_ = os.RemoveAll(dst)
	_ = os.MkdirAll(dst, 0o755)
	if err := jsonx.WriteAtomic(filepath.Join(dst, "plugin.json"), model.PluginManifest{Schema: "https://antigravity.google/schemas/v1/plugin.json", Name: safeName(name), Description: "Imported by agctl"}, ""); err != nil {
		return Item{}, err
	}
	found := 0
	for _, d := range []string{"skills", "agents", "rules", "sidecars"} {
		if st, e := os.Stat(filepath.Join(cache, d)); e == nil && st.IsDir() {
			if er := copyTreeMerge(filepath.Join(cache, d), filepath.Join(dst, d)); er != nil {
				return Item{}, er
			}
			found++
		}
	}
	for _, f := range []string{"mcp_config.json", "hooks.json"} {
		if st, e := os.Stat(filepath.Join(cache, f)); e == nil && !st.IsDir() {
			if er := copyFile(filepath.Join(cache, f), filepath.Join(dst, f)); er != nil {
				return Item{}, er
			}
			found++
		}
	}
	if found == 0 {
		_ = os.RemoveAll(dst)
		return Item{}, fmt.Errorf("repository has no plugin-compatible skills/agents/rules/sidecars/mcp_config.json/hooks.json")
	}
	if strings.TrimSpace(workspace) == "" {
		if err := mirrorToCLI(p, dst, safeName(name)); err != nil {
			return Item{}, fmt.Errorf("mirror plugin to Antigravity CLI: %w", err)
		}
	}
	_ = writeLock(p, safeName(name), "plugin", repo, ref, commit, dst)
	return inspect(dst, scope(workspace)), nil
}

func SetCLIEnabled(name string, enabled bool) error {
	name = safeName(name)
	if name == "" {
		return fmt.Errorf("invalid plugin name")
	}
	if !execx.Exists("agy") {
		return fmt.Errorf("agy CLI is required to change CLI plugin enabled state")
	}
	action := "disable"
	if enabled {
		action = "enable"
	}
	out, err := execx.Run(60*time.Second, "", "agy", "plugin", action, name)
	if err != nil {
		return fmt.Errorf("agy plugin %s %s: %w: %s", action, name, err, strings.TrimSpace(out))
	}
	return nil
}

func Remove(p paths.Paths, workspace, name string) error {
	name = safeName(name)
	if name == "" {
		return fmt.Errorf("invalid plugin name")
	}
	if strings.TrimSpace(workspace) == "" && execx.Exists("agy") {
		// Official CLI uninstall also cleans CLI plugin registries. Ignore only a
		// not-installed failure and still remove the IDE-side managed copy below.
		_, _ = execx.Run(60*time.Second, "", "agy", "plugin", "uninstall", name)
	}
	if err := os.RemoveAll(filepath.Join(Root(p, workspace), name)); err != nil {
		return err
	}
	if strings.TrimSpace(workspace) == "" && p.CLIPluginsRoot != "" {
		if err := os.RemoveAll(filepath.Join(p.CLIPluginsRoot, name)); err != nil {
			return err
		}
	}
	return nil
}

func Doctor(p paths.Paths, workspace string) []Item {
	xs, _ := List(p, workspace)
	if strings.TrimSpace(workspace) == "" && p.CLIPluginsRoot != "" {
		for i := range xs {
			if xs[i].Scope == "cli-global" {
				continue
			}
			cliPath := filepath.Join(p.CLIPluginsRoot, safeName(xs[i].Name))
			cliItem := inspect(cliPath, "cli-global")
			if !cliItem.Valid {
				xs[i].Issues = append(xs[i].Issues, "CLI mirror invalid/missing: "+strings.Join(cliItem.Issues, "; "))
				// An IDE-only plugin remains valid for the IDE, but report the CLI gap.
			}
		}
	}
	return xs
}

func scope(workspace string) string {
	if strings.TrimSpace(workspace) == "" {
		return "global"
	}
	return "workspace"
}

func unionStrings(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, xs := range [][]string{a, b} {
		for _, x := range xs {
			if x != "" && !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	sort.Strings(out)
	return out
}

var pluginNameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func safeName(s string) string {
	s = strings.TrimSpace(s)
	if !pluginNameRE.MatchString(s) {
		return ""
	}
	return s
}

func mirrorToCLI(p paths.Paths, idePath, name string) error {
	if p.CLIPluginsRoot == "" {
		return nil
	}
	name = safeName(name)
	if name == "" {
		return fmt.Errorf("plugin name is not CLI-compatible")
	}
	stage := filepath.Join(p.AppRoot, "cli-plugin-staging", name)
	if err := copyTreeReplace(idePath, stage); err != nil {
		return err
	}
	manifestPath := filepath.Join(stage, "plugin.json")
	raw, err := jsonx.ReadMap(manifestPath)
	if err != nil {
		return err
	}
	// CLI v1 manifest schema allows only name/description (plus $schema).
	description, _ := raw["description"].(string)
	cliManifest := model.PluginManifest{Schema: "https://antigravity.google/schemas/v1/plugin.json", Name: name, Description: description}
	if err := jsonx.WriteAtomic(manifestPath, cliManifest, p.BackupsRoot); err != nil {
		return err
	}
	if execx.Exists("agy") {
		// Use the documented CLI installer so its plugin registry and enabled-state
		// bookkeeping stay consistent.
		_, _ = execx.Run(30*time.Second, "", "agy", "plugin", "uninstall", name)
		out, err := execx.Run(60*time.Second, "", "agy", "plugin", "install", stage)
		if err != nil {
			return fmt.Errorf("agy plugin install %s: %w: %s", name, err, strings.TrimSpace(out))
		}
		return nil
	}
	// Fallback only for build/test environments where AGY is unavailable.
	return copyTreeReplace(stage, filepath.Join(p.CLIPluginsRoot, name))
}

func shortHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:8]) }

func writeLock(p paths.Paths, id, kind, source, ref, commit, root string) error {
	files := map[string]string{}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return nil
		}
		h := sha256.Sum256(b)
		rel, _ := filepath.Rel(root, path)
		files[filepath.ToSlash(rel)] = hex.EncodeToString(h[:])
		return nil
	})
	lock := model.ProvenanceLock{ID: id, Kind: kind, Source: source, Ref: ref, Commit: commit, InstalledAt: time.Now().Format(time.RFC3339Nano), Path: root, Files: files}
	return jsonx.WriteAtomic(filepath.Join(p.LocksRoot, kind+"-"+safeName(id)+".lock.json"), lock, p.BackupsRoot)
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
		rel, e := filepath.Rel(src, path)
		if e != nil {
			return e
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, target)
	})
}
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.Create(dst)
	if e != nil {
		return e
	}
	_, cp := io.Copy(out, in)
	ce := out.Close()
	if cp != nil {
		return cp
	}
	return ce
}
