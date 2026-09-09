package transcript

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const samplePath = "testdata/sample.jsonl"

// lineOffsets recomputes the byte offset of each line independently of the
// adapter, so span provenance is verified against the file, not the code.
func lineOffsets(t *testing.T, data string) []int {
	t.Helper()
	offsets := []int{}
	off := 0
	for _, line := range strings.Split(strings.TrimSuffix(data, "\n"), "\n") {
		offsets = append(offsets, off)
		off += len(line) + 1
	}
	return offsets
}

func TestLoadLiftsAssistantTextSpans(t *testing.T) {
	data, err := os.ReadFile(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := Load(samplePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sess.SessionID != "9f3c2a10-6d1b-4c2e-8f0a-3b7d2c9e1a44" {
		t.Errorf("SessionID = %q, want the sessionId carried by the records", sess.SessionID)
	}
	if got := len(sess.Lines); got != 20 {
		t.Errorf("Lines = %d, want 20", got)
	}
	// Assistant text spans exist on lines 3, 7, 13, 18 and 20. The sidechain
	// record on line 11, the tool_use/tool_result records, and every
	// non-message record (mode, file-history-snapshot, queue-operation,
	// custom-title, attachment, system, last-prompt) must produce no span.
	wantLines := []int{3, 7, 13, 18, 20}
	if len(sess.Spans) != len(wantLines) {
		t.Fatalf("Spans = %d, want %d", len(sess.Spans), len(wantLines))
	}
	offsets := lineOffsets(t, string(data))
	for i, want := range wantLines {
		s := sess.Spans[i]
		if s.LineStart != want || s.LineEnd != want {
			t.Errorf("span %d lines = %d-%d, want %d", i, s.LineStart, s.LineEnd, want)
		}
		if s.ByteStart != offsets[want-1] {
			t.Errorf("span %d ByteStart = %d, want %d", i, s.ByteStart, offsets[want-1])
		}
		if s.ByteEnd != offsets[want-1]+len(sess.Lines[want-1]) {
			t.Errorf("span %d ByteEnd = %d, want end of line %d", i, s.ByteEnd, want)
		}
		if s.MessageUUID == "" || s.Timestamp == "" || !strings.HasSuffix(s.Timestamp, "Z") {
			t.Errorf("span %d provenance incomplete: uuid=%q timestamp=%q", i, s.MessageUUID, s.Timestamp)
		}
		if s.Text == "" {
			t.Errorf("span %d has empty text", i)
		}
	}
	last := sess.Spans[len(sess.Spans)-1]
	if !strings.Contains(last.Text, "In summary") {
		t.Errorf("last span text = %q, want the final summary record", last.Text)
	}
	if sess.Spans[3].Text != "Should I also refactor the config loader while I'm in here? It would touch 3 more files." {
		t.Errorf("span 3 text = %q, want the permission question", sess.Spans[3].Text)
	}
	if !strings.Contains(sess.Spans[2].Text, "总结") {
		t.Errorf("span 2 text = %q, want the Chinese mid-run conclusion", sess.Spans[2].Text)
	}
}

func TestLoadSkipsPartialTrailingLine(t *testing.T) {
	data, err := os.ReadFile(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a record the harness is still appending: cut the last line
	// in half, without a trailing newline.
	truncated := strings.TrimSuffix(string(data), "\n")
	cut := truncated[:len(truncated)-40]
	path := filepath.Join(t.TempDir(), "partial.jsonl")
	if err := os.WriteFile(path, []byte(cut), 0o644); err != nil {
		t.Fatal(err)
	}
	sess, err := Load(path)
	if err != nil {
		t.Fatalf("Load on partial file: %v", err)
	}
	if n := len(sess.Spans); n != 4 {
		t.Errorf("Spans = %d, want 4 (partial line skipped)", n)
	}
}

func TestLoadEmptyFileFallsBackToBaseNameID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "0d1e2f30-1111-4222-8333-444455556666.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	sess, err := Load(path)
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if sess.SessionID != "0d1e2f30-1111-4222-8333-444455556666" {
		t.Errorf("SessionID = %q, want file base name fallback", sess.SessionID)
	}
	if len(sess.Spans) != 0 {
		t.Errorf("Spans = %d, want 0", len(sess.Spans))
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Fatal("Load on missing file should error")
	}
}

func TestWatchSendsSessionOnAppend(t *testing.T) {
	data, err := os.ReadFile(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "live.jsonl")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	initial, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := Watch(ctx, path, 10*time.Millisecond)

	appended := `{"type":"assistant","uuid":"c3d4e5f6-0000-4000-8000-000000000099","parentUuid":"a1b0c2d3-0000-4000-8000-000000000013","sessionId":"9f3c2a10-6d1b-4c2e-8f0a-3b7d2c9e1a44","timestamp":"2026-09-08T09:17:01.000Z","isSidechain":false,"message":{"role":"assistant","type":"message","content":[{"type":"text","text":"Follow-up: the fix also holds under -race."}]}}
`
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(appended); err != nil {
		t.Fatal(err)
	}
	f.Close()

	select {
	case sess, ok := <-updates:
		if !ok {
			t.Fatal("updates channel closed unexpectedly")
		}
		if len(sess.Spans) != len(initial.Spans)+1 {
			t.Errorf("updated session has %d spans, want %d", len(sess.Spans), len(initial.Spans)+1)
		}
		if last := sess.Spans[len(sess.Spans)-1]; !strings.Contains(last.Text, "Follow-up") {
			t.Errorf("last span = %q, want the appended record", last.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no update received after append")
	}
	cancel()
	select {
	case _, ok := <-updates:
		if ok {
			t.Error("channel should close after context cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Error("channel did not close after context cancellation")
	}
}
