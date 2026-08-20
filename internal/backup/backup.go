package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/paths"
)

func CreateAll(p paths.Paths, workspace string) ([]string, error) {
	files := []string{p.CLISettings, p.GlobalConfig, p.GlobalMCP, p.GlobalHooks, p.GlobalRule, p.RouterConfig, p.LoopConfig}
	if workspace != "" {
		files = append(files, paths.WorkspaceMCP(workspace))
	}
	var out []string
	for _, f := range files {
		if _, err := os.Stat(f); err == nil {
			b, err := jsonx.BackupFile(f, p.BackupsRoot)
			if err != nil {
				return out, err
			}
			out = append(out, b)
		}
	}
	return out, nil
}

func List(p paths.Paths) ([]string, error) { return jsonx.ListBackups(p.BackupsRoot) }

func Restore(p paths.Paths, backupPath, destination string) error {
	if !filepath.IsAbs(backupPath) {
		backupPath = filepath.Join(p.BackupsRoot, backupPath)
	}
	if destination == "" {
		base := filepath.Base(backupPath)
		idx := strings.Index(base, ".20")
		if idx <= 0 {
			return fmt.Errorf("destination required; cannot infer from %s", base)
		}
		original := base[:idx]
		candidates := map[string]string{
			"settings.json": p.CLISettings, "config.json": p.GlobalConfig, "mcp_config.json": p.GlobalMCP, "hooks.json": p.GlobalHooks,
			"GEMINI.md": p.GlobalRule, "router.json": p.RouterConfig, "loop.json": p.LoopConfig,
		}
		destination = candidates[original]
		if destination == "" {
			return fmt.Errorf("destination required for backup %s", base)
		}
	}
	return jsonx.RestoreBackup(backupPath, destination)
}
