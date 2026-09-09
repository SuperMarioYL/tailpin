package transcript

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrNoSessions is returned when the projects root exists but holds no
// top-level session files to watch.
var ErrNoSessions = errors.New("no session transcripts found")

// Ref identifies one discovered session file.
type Ref struct {
	Path      string
	SessionID string
	Project   string // project directory name, e.g. "-Users-dev-work-api-server"
	ModTime   time.Time
}

// DefaultRoot returns the Claude Code projects root: $CLAUDE_CONFIG_DIR/projects
// when the harness was started with a config override, otherwise
// ~/.claude/projects.
func DefaultRoot() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "projects"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// Sessions lists every top-level session file under root, newest first.
// Only *.jsonl files directly inside a project directory are considered:
// nested subagent traces and journals (written under
// <session-id>/subagents/...) and other subdirectory content never surface,
// so discovery cannot pick a subagent file as the session.
func Sessions(root string) ([]Ref, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read sessions root %s: %w", root, err)
	}
	var refs []Ref
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		project := entry.Name()
		projectDir := filepath.Join(root, project)
		files, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			info, err := f.Info()
			if err != nil {
				continue
			}
			path := filepath.Join(projectDir, f.Name())
			refs = append(refs, Ref{
				Path:      path,
				SessionID: strings.TrimSuffix(f.Name(), ".jsonl"),
				Project:   project,
				ModTime:   info.ModTime(),
			})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].ModTime.After(refs[j].ModTime)
	})
	return refs, nil
}

// Latest returns the most recently modified top-level session file under
// root — the session the harness is presumably still writing to.
func Latest(root string) (Ref, error) {
	refs, err := Sessions(root)
	if err != nil {
		return Ref{}, err
	}
	if len(refs) == 0 {
		return Ref{}, fmt.Errorf("%w under %s", ErrNoSessions, root)
	}
	return refs[0], nil
}
