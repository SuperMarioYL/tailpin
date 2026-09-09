package transcript

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("{\"type\":\"user\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setMTime(t *testing.T, path string, mod time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestSessionsSkipsNestedAndSortsNewestFirst(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)

	projA := filepath.Join(root, "-Users-dev-work-api-server")
	projB := filepath.Join(root, "-Users-dev-work-other")
	nested := filepath.Join(projA, "9f3c2a10-6d1b-4c2e-8f0a-3b7d2c9e1a44", "subagents", "workflows", "wf_1")
	memory := filepath.Join(projA, "memory")
	for _, dir := range []string{projA, projB, nested, memory} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	older := filepath.Join(projA, "aaa11111-0000-4000-8000-000000000001.jsonl")
	newest := filepath.Join(projA, "bbb22222-0000-4000-8000-000000000002.jsonl")
	other := filepath.Join(projB, "ddd44444-0000-4000-8000-000000000004.jsonl")
	subagent := filepath.Join(nested, "agent-1.jsonl")
	journal := filepath.Join(nested, "journal.jsonl")
	memFile := filepath.Join(memory, "notes.jsonl")
	stray := filepath.Join(root, "stray.jsonl")
	for _, p := range []string{older, newest, other, subagent, journal, memFile, stray} {
		writeFile(t, p)
	}
	setMTime(t, older, base.Add(1*time.Minute))
	setMTime(t, newest, base.Add(3*time.Minute))
	setMTime(t, other, base.Add(2*time.Minute))
	setMTime(t, subagent, base.Add(9*time.Minute)) // newest on disk, but nested
	setMTime(t, journal, base.Add(9*time.Minute))
	setMTime(t, memFile, base.Add(9*time.Minute))
	setMTime(t, stray, base.Add(9*time.Minute))

	refs, err := Sessions(root)
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("Sessions returned %d refs, want 3 top-level files only: %+v", len(refs), refs)
	}
	wantOrder := []string{"bbb22222-0000-4000-8000-000000000002", "ddd44444-0000-4000-8000-000000000004", "aaa11111-0000-4000-8000-000000000001"}
	for i, want := range wantOrder {
		if refs[i].SessionID != want {
			t.Errorf("refs[%d].SessionID = %q, want %q", i, refs[i].SessionID, want)
		}
	}
	if refs[0].Project != "-Users-dev-work-api-server" {
		t.Errorf("refs[0].Project = %q", refs[0].Project)
	}
	if refs[0].Path != newest {
		t.Errorf("refs[0].Path = %q, want %q", refs[0].Path, newest)
	}
}

func TestLatest(t *testing.T) {
	root := t.TempDir()
	if _, err := Latest(root); err == nil {
		t.Fatal("Latest on empty root should error")
	}
	proj := filepath.Join(root, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(proj, "aaa.jsonl")
	b := filepath.Join(proj, "bbb.jsonl")
	writeFile(t, a)
	writeFile(t, b)
	base := time.Now()
	setMTime(t, a, base.Add(-2*time.Hour))
	setMTime(t, b, base.Add(-1*time.Hour))
	ref, err := Latest(root)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if ref.Path != b {
		t.Errorf("Latest = %q, want the more recently modified %q", ref.Path, b)
	}
}

func TestSessionsMissingRoot(t *testing.T) {
	_, err := Sessions(filepath.Join(t.TempDir(), "nope"))
	if err == nil || errors.Is(err, ErrNoSessions) {
		t.Fatalf("missing root must return a read error, got %v", err)
	}
}

func TestDefaultRoot(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/cc-config")
	root, err := DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root != "/tmp/cc-config/projects" {
		t.Errorf("DefaultRoot with CLAUDE_CONFIG_DIR = %q", root)
	}
}
