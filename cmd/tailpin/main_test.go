package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SuperMarioYL/tailpin/internal/pin"
	"github.com/SuperMarioYL/tailpin/internal/transcript"
)

const sampleFixture = "../../internal/transcript/testdata/sample.jsonl"

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		var out bytes.Buffer
		if err := run(context.Background(), []string{arg}, &out, &bytes.Buffer{}); err != nil {
			t.Fatalf("%s: %v", arg, err)
		}
		for _, want := range []string{"tailpin watch <file> --plain", "tailpin diff [idA idB]", "j/k move"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s output missing %q:\n%s", arg, want, out.String())
			}
		}
	}
}

func TestRunVersion(t *testing.T) {
	for _, arg := range []string{"version", "-v", "--version"} {
		var out bytes.Buffer
		if err := run(context.Background(), []string{arg}, &out, &bytes.Buffer{}); err != nil {
			t.Fatalf("%s: %v", arg, err)
		}
		if got := out.String(); got != "tailpin "+version+"\n" {
			t.Errorf("%s output = %q", arg, got)
		}
	}
}

func TestRunUnknownCommand(t *testing.T) {
	err := run(context.Background(), []string{"frobnicate"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("unknown command err = %v", err)
	}
}

func TestRunWatchErrors(t *testing.T) {
	err := run(context.Background(), []string{"watch"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "requires a session file") {
		t.Errorf("watch without file err = %v", err)
	}
	err = run(context.Background(), []string{"watch", "/nonexistent/session.jsonl"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "open session file") {
		t.Errorf("watch missing file err = %v", err)
	}
}

// waitUntil polls cond until it holds or the timeout elapses.
func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

// syncBuffer is a bytes.Buffer safe for concurrent use: watchPlain renders
// from its own goroutine while the test polls the output.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestWatchPlainRendersAnswerAndLeavesTranscriptUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "9f3c2a10-6d1b-4c2e-8f0a-3b7d2c9e1a44.jsonl")
	original, err := os.ReadFile(sampleFixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var out syncBuffer
	done := make(chan error, 1)
	go func() {
		done <- watchPlain(ctx, path, filepath.Join(dir, "pins"), &out, false)
	}()
	waitUntil(t, 2*time.Second, func() bool { return strings.Contains(out.String(), "In summary") })
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("watchPlain: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"9f3c2a10-6d1b-4c2e-8f0a-3b7d2c9e1a44",
		"rev 1",
		"L20",
		"Root cause: countTokens in internal/parser/parser.go double-counted",
		"Caveat: the config loader refactor is still pending.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plain output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Error("piped plain output must not contain ANSI escapes")
	}

	// Read-only acceptance: the transcript is byte-identical.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, after) {
		t.Error("watchPlain modified the transcript")
	}

	// The pin artifact for the session was written under the pin store.
	if _, err := os.Stat(filepath.Join(dir, "pins", "9f3c2a10-6d1b-4c2e-8f0a-3b7d2c9e1a44.pin.json")); err != nil {
		t.Errorf("pin artifact: %v", err)
	}
}

func TestRenderPlainWithoutAnswer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-answer.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	sess, err := transcript.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := pin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	renderPlain(&out, sess, pin.NewTracker(store, sess.SessionID), false)
	if got := out.String(); !strings.Contains(got, "waiting for the agent's first conclusion") {
		t.Errorf("renderPlain without answer = %q", got)
	}
}

// appendRecord appends one assistant text record to a session file, the way
// a harness appends a batch mid-run.
func appendRecord(t *testing.T, path, uuid, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	textJSON, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	record := fmt.Sprintf(`{"type":"assistant","uuid":%q,"sessionId":"live-1","timestamp":"2026-09-08T10:00:00.000Z","message":{"role":"assistant","content":[{"type":"text","text":%s}]}}`+"\n", uuid, textJSON)
	if _, err := f.WriteString(record); err != nil {
		t.Fatal(err)
	}
}

func TestWatchPlainUpdatesWithinASecondOfAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "live-1.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var out syncBuffer
	done := make(chan error, 1)
	go func() {
		done <- watchPlain(ctx, path, filepath.Join(dir, "pins"), &out, false)
	}()
	waitUntil(t, 2*time.Second, func() bool { return strings.Contains(out.String(), "waiting for the agent") })

	start := time.Now()
	appendRecord(t, path, "u-1", "In summary: the flaky parser test is fixed and verified with go test ./...")
	waitUntil(t, 2*time.Second, func() bool { return strings.Contains(out.String(), "the flaky parser test is fixed") })
	elapsed := time.Since(start)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("watchPlain: %v", err)
	}

	// The plan's acceptance: the answer refreshes within ~1s of an append
	// (250ms poll); allow slack for slow CI.
	if elapsed > 2*time.Second {
		t.Errorf("answer took %v to refresh after append, want < 2s", elapsed)
	}
	if !strings.Contains(out.String(), "rev 1") {
		t.Errorf("appended answer should be revision 1:\n%s", out.String())
	}
}

// sessionFile names one session file in a project directory with an explicit
// mtime, so discovery order is deterministic.
type sessionFile struct {
	project string
	name    string
	mod     time.Time
}

// layoutMtimes stamps mtimes onto session files under root (creating empty
// files where none exist). Call it after writing file contents: writing
// updates the mtime.
func layoutMtimes(t *testing.T, root string, files ...sessionFile) {
	t.Helper()
	for _, f := range files {
		dir := filepath.Join(root, f.project)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, f.name)
		if _, err := os.Stat(path); err != nil {
			if err := os.WriteFile(path, nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Chtimes(path, f.mod, f.mod); err != nil {
			t.Fatal(err)
		}
	}
}

// writeAnswerFile writes one assistant text record as a session file.
func writeAnswerFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	textJSON, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	record := fmt.Sprintf(`{"type":"assistant","uuid":"u-1","sessionId":%q,"timestamp":"2026-09-08T09:00:00.000Z","message":{"role":"assistant","content":[{"type":"text","text":%s}]}}`+"\n", sessionID, textJSON)
	if err := os.WriteFile(path, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiffBetweenTwoExplicitSessions(t *testing.T) {
	base := time.Now().Add(-time.Hour)
	root := t.TempDir()
	apiDir := filepath.Join(root, "-Users-dev-work-api-server")
	writeAnswerFile(t, filepath.Join(apiDir, "run-a.jsonl"),
		"In summary: the root cause is the parser loop boundary.")
	writeAnswerFile(t, filepath.Join(apiDir, "run-b.jsonl"),
		"In summary: the root cause is the lexer token limit.")
	layoutMtimes(t, root,
		sessionFile{"-Users-dev-work-api-server", "run-a.jsonl", base},
		sessionFile{"-Users-dev-work-api-server", "run-b.jsonl", base.Add(time.Minute)},
	)

	var out bytes.Buffer
	if err := diffSessions(root, t.TempDir(), []string{"run-a", "run-b"}, &out, false); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"run-a -> run-b", "~ changed", "parser loop boundary", "lexer token limit"} {
		if !strings.Contains(got, want) {
			t.Errorf("diff output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Error("plain diff output must not contain ANSI escapes")
	}
}

func TestDiffNewestAgainstPreviousInSameProject(t *testing.T) {
	base := time.Now().Add(-time.Hour)
	root := t.TempDir()
	apiDir := filepath.Join(root, "-Users-dev-work-api-server")
	writeAnswerFile(t, filepath.Join(apiDir, "old-run.jsonl"), "In summary: the bug is fixed in parser.go.")
	writeAnswerFile(t, filepath.Join(apiDir, "new-run.jsonl"), "In summary: the bug is fixed in parser.go and lexer.go.")
	writeAnswerFile(t, filepath.Join(root, "-Users-dev-work-other", "unrelated.jsonl"), "Unrelated work.")
	layoutMtimes(t, root,
		sessionFile{"-Users-dev-work-api-server", "old-run.jsonl", base},
		sessionFile{"-Users-dev-work-other", "unrelated.jsonl", base.Add(time.Minute)},
		sessionFile{"-Users-dev-work-api-server", "new-run.jsonl", base.Add(2 * time.Minute)},
	)

	var out bytes.Buffer
	if err := diffSessions(root, t.TempDir(), nil, &out, false); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	// The newest session is new-run; the previous session in the SAME project
	// is old-run, even though unrelated.jsonl is newer than old-run.
	if !strings.Contains(got, "old-run -> new-run") {
		t.Errorf("diff should compare old-run -> new-run:\n%s", got)
	}
	if strings.Contains(got, "unrelated") {
		t.Errorf("diff must not pick a session from another project:\n%s", got)
	}
}

func TestDiffNoPreviousSessionInProject(t *testing.T) {
	root := t.TempDir()
	layoutMtimes(t, root,
		sessionFile{"-Users-dev-work-api-server", "only-run.jsonl", time.Now()},
	)
	err := diffSessions(root, t.TempDir(), nil, &bytes.Buffer{}, false)
	if err == nil || !strings.Contains(err.Error(), "no previous session") {
		t.Errorf("single-session project err = %v", err)
	}
}

func TestDiffUnknownSessionID(t *testing.T) {
	root := t.TempDir()
	layoutMtimes(t, root,
		sessionFile{"-Users-dev-work-api-server", "run-a.jsonl", time.Now()},
	)
	err := diffSessions(root, t.TempDir(), []string{"run-a", "nope"}, &bytes.Buffer{}, false)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("unknown id err = %v", err)
	}
}

func TestDiffPrefersStoredArtifactOverExtraction(t *testing.T) {
	base := time.Now().Add(-time.Hour)
	root := t.TempDir()
	apiDir := filepath.Join(root, "-Users-dev-work-api-server")
	writeAnswerFile(t, filepath.Join(apiDir, "run-a.jsonl"), "In summary: the transcript says parser.go.")
	writeAnswerFile(t, filepath.Join(apiDir, "run-b.jsonl"), "In summary: the transcript says parser.go.")
	layoutMtimes(t, root,
		sessionFile{"-Users-dev-work-api-server", "run-a.jsonl", base},
		sessionFile{"-Users-dev-work-api-server", "run-b.jsonl", base.Add(time.Minute)},
	)

	// The stored artifact for run-a differs from its transcript.
	storeDir := t.TempDir()
	store, err := pin.NewStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	stored := &pin.PinnedAnswer{
		SessionID:  "run-a",
		AnswerText: "In summary: the pinned artifact says lexer.go.",
		Confidence: pin.ConfidenceHigh,
		Claims:     []pin.Claim{{Text: "In summary: the pinned artifact says lexer.go.", Kind: pin.KindFinding}},
	}
	if err := store.Save(stored); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := diffSessions(root, storeDir, []string{"run-a", "run-b"}, &out, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "pinned artifact says lexer.go") {
		t.Errorf("diff should use the stored artifact for run-a:\n%s", out.String())
	}
}

func TestRunDiffUsageErrors(t *testing.T) {
	// Usage errors must not depend on the machine's sessions root: a clean
	// CI runner has no ~/.claude/projects at all, so point the projects
	// root at a directory that does not exist.
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(t.TempDir(), "absent"))
	err := run(context.Background(), []string{"diff", "only-one"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "two") {
		t.Errorf("one id err = %v", err)
	}
	err = run(context.Background(), []string{"diff", "a", "b", "c"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Errorf("three ids err = %v", err)
	}
}

func TestIsTTY(t *testing.T) {
	if isTTY(&bytes.Buffer{}) {
		t.Error("a bytes.Buffer is not a TTY")
	}
}

func TestLineRange(t *testing.T) {
	if got := lineRange(pin.Span{LineStart: 20, LineEnd: 20}); got != "L20" {
		t.Errorf("lineRange = %q", got)
	}
	if got := lineRange(pin.Span{LineStart: 20, LineEnd: 22}); got != "L20-22" {
		t.Errorf("lineRange = %q", got)
	}
}
