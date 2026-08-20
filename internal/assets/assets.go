package assets

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed data/skills
var data embed.FS

func SkillNames() []string {
	return []string{
		"adaptive-tool-router",
		"autonomous-engineering",
		"autonomous-completion-loop",
		"editorial-quality-director",
	}
}

func InstallSkill(name, dstRoot string) error {
	srcRoot := "data/skills/" + name
	if _, err := fs.Stat(data, srcRoot); err != nil {
		return fmt.Errorf("embedded skill %q: %w", name, err)
	}
	dst := filepath.Join(dstRoot, name)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(data, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, srcRoot)
		rel = strings.TrimPrefix(rel, "/")
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := data.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

func ReadSkillFile(name, rel string) ([]byte, error) {
	return data.ReadFile(filepath.ToSlash(filepath.Join("data/skills", name, rel)))
}
