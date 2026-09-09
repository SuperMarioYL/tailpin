package pin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testAnswer(sessionID, text string) *PinnedAnswer {
	return &PinnedAnswer{
		SessionID:  sessionID,
		AnswerText: text,
		Confidence: ConfidenceMed,
		Claims: []Claim{
			{Text: text, Kind: KindFinding, Span: Span{
				File: "/tmp/sess.jsonl", LineStart: 3, LineEnd: 3,
				ByteStart: 210, ByteEnd: 480,
				MessageUUID: "u-1", Timestamp: "2026-09-08T09:15:03.456Z",
			}},
		},
	}
}

func TestStoreSaveLoadRoundtrip(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pa := testAnswer("sess-a", "The fix landed in parser.go.")
	pa.Revision = 2
	pa.UpdatedAt = time.Date(2026, 9, 8, 9, 16, 40, 0, time.UTC)
	if err := store.Save(pa); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(filepath.Join(store.dir, "sess-a.pin.json")); err != nil {
		t.Errorf("artifact file: %v", err)
	}
	loaded, err := store.Load("sess-a")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.SessionID != pa.SessionID || loaded.Revision != pa.Revision ||
		!loaded.UpdatedAt.Equal(pa.UpdatedAt) || loaded.AnswerText != pa.AnswerText ||
		loaded.Confidence != pa.Confidence || len(loaded.Claims) != 1 {
		t.Fatalf("roundtrip mismatch: %+v", loaded)
	}
	c := loaded.Claims[0]
	if c.Text != pa.Claims[0].Text || c.Kind != KindFinding {
		t.Errorf("claim mismatch: %+v", c)
	}
	s := c.Span
	if s.File != "/tmp/sess.jsonl" || s.LineStart != 3 || s.LineEnd != 3 ||
		s.ByteStart != 210 || s.ByteEnd != 480 || s.MessageUUID != "u-1" ||
		s.Timestamp != "2026-09-08T09:15:03.456Z" {
		t.Errorf("span mismatch: %+v", s)
	}
}

func TestStoreSchemaFieldNames(t *testing.T) {
	// The artifact is the plan's cross-run diff contract, so its JSON field
	// names must stay exactly as specified.
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(testAnswer("sess-schema", "x")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.Path("sess-schema"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"session_id", "revision", "updated_at", "answer_text", "confidence", "claims"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("artifact missing top-level key %q", key)
		}
	}
	claim, ok := raw["claims"].([]any)
	if !ok || len(claim) != 1 {
		t.Fatalf("claims shape wrong: %v", raw["claims"])
	}
	for _, key := range []string{"text", "kind", "span"} {
		if _, ok := claim[0].(map[string]any)[key]; !ok {
			t.Errorf("claim missing key %q", key)
		}
	}
	span := claim[0].(map[string]any)["span"].(map[string]any)
	for _, key := range []string{"file", "line_start", "line_end", "byte_start", "byte_end", "message_uuid", "timestamp"} {
		if _, ok := span[key]; !ok {
			t.Errorf("span missing key %q", key)
		}
	}
}

func TestStoreLoadNotFound(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Load("never-pinned")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Load unknown session err = %v, want ErrNotFound", err)
	}
}

func TestStoreSaveLeavesNoTempFiles(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := store.Save(testAnswer("sess-atomic", "revision text")); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "sess-atomic.pin.json" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("store dir = %v, want only the final artifact (atomic rename cleaned up temps)", names)
	}
}

func TestTrackerBumpsRevisionOnAnswerChange(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tr := NewTracker(store, "sess-rev")

	first := testAnswer("sess-rev", "first answer")
	if err := tr.Observe(first); err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 {
		t.Errorf("first observation revision = %d, want 1", first.Revision)
	}

	same := testAnswer("sess-rev", "first answer")
	if err := tr.Observe(same); err != nil {
		t.Fatal(err)
	}
	if same.Revision != 0 {
		t.Errorf("unchanged answer should not be assigned a new revision, got %d", same.Revision)
	}

	changed := testAnswer("sess-rev", "second answer with more detail")
	if err := tr.Observe(changed); err != nil {
		t.Fatal(err)
	}
	if changed.Revision != 2 {
		t.Errorf("changed answer revision = %d, want 2", changed.Revision)
	}
	if changed.UpdatedAt.IsZero() {
		t.Error("changed answer must carry updated_at")
	}

	// A restart must continue the revision history from disk.
	restarted := NewTracker(store, "sess-rev")
	third := testAnswer("sess-rev", "third answer")
	if err := restarted.Observe(third); err != nil {
		t.Fatal(err)
	}
	if third.Revision != 3 {
		t.Errorf("revision after restart = %d, want 3", third.Revision)
	}
	persisted, err := store.Load("sess-rev")
	if err != nil {
		t.Fatal(err)
	}
	if persisted.AnswerText != "third answer" || persisted.Revision != 3 {
		t.Errorf("persisted artifact = rev %d text %q", persisted.Revision, persisted.AnswerText)
	}
}

func TestTrackerNilAnswerIsNoop(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tr := NewTracker(store, "sess-nil")
	if err := tr.Observe(nil); err != nil {
		t.Fatal(err)
	}
	if tr.Revision() != 0 {
		t.Errorf("revision = %d, want 0", tr.Revision())
	}
	if _, err := store.Load("sess-nil"); !errors.Is(err, ErrNotFound) {
		t.Errorf("nil answer must not write an artifact, err = %v", err)
	}
}

func TestAnswerTextIsClaimsJoined(t *testing.T) {
	pa := testAnswer("sess-join", "line one")
	if !strings.Contains(pa.AnswerText, pa.Claims[0].Text) {
		t.Error("answer text should contain the claim text")
	}
}
