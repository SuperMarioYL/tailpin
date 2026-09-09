// Package pin owns the pinned-answer primitive: the conclusion extracted
// from a session transcript becomes a separately addressable, revisioned
// artifact with claim-level provenance. Extractors are deterministic (no
// model calls); the store writes only Tailpin's own files under
// ~/.tailpin/sessions and never touches the harness's transcript.
package pin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Confidence is the extractor's marker-score verdict for an answer.
type Confidence string

const (
	ConfidenceLow  Confidence = "low"
	ConfidenceMed  Confidence = "med"
	ConfidenceHigh Confidence = "high"
)

// Kind classifies a single claim.
type Kind string

const (
	KindFinding Kind = "finding"
	KindPlan    Kind = "plan"
	KindFix     Kind = "fix"
	KindCaveat  Kind = "caveat"
)

// Span is the persisted provenance of a claim: where in the raw transcript
// the claim came from. Line numbers are 1-based and inclusive; byte offsets
// are 0-based file offsets of the producing record (excluding the line
// terminator); timestamp is ISO 8601 with a Z suffix as written by the
// harness.
type Span struct {
	File        string `json:"file"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end"`
	ByteStart   int    `json:"byte_start"`
	ByteEnd     int    `json:"byte_end"`
	MessageUUID string `json:"message_uuid"`
	Timestamp   string `json:"timestamp"`
}

// Claim is one sentence or bullet of the pinned answer.
type Claim struct {
	Text string `json:"text"`
	Kind Kind   `json:"kind"`
	Span Span   `json:"span"`
}

// PinnedAnswer is the current best answer of one session: a short,
// claim-by-claim conclusion instead of the transcript tail. AnswerText is
// the claims joined by newlines and stays within roughly 40 lines.
type PinnedAnswer struct {
	SessionID  string     `json:"session_id"`
	Revision   int        `json:"revision"` // bumped on every answer change
	UpdatedAt  time.Time  `json:"updated_at"`
	AnswerText string     `json:"answer_text"`
	Confidence Confidence `json:"confidence"`
	Claims     []Claim    `json:"claims"`
}

// ErrNotFound reports a session with no stored pin artifact.
var ErrNotFound = errors.New("no pinned answer stored for session")

// Store persists one pin artifact per session under dir, written atomically
// so a kill mid-write can never leave a half-written artifact behind.
type Store struct {
	dir string
}

// NewStore creates the artifact directory if needed.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create pin store: %w", err)
	}
	return &Store{dir: dir}, nil
}

// DefaultDir is the artifact store location from the plan:
// ~/.tailpin/sessions.
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tailpin", "sessions"), nil
}

// Path returns the artifact file path for a session.
func (s *Store) Path(sessionID string) string {
	return filepath.Join(s.dir, sessionID+".pin.json")
}

// Load reads the stored artifact for a session. It returns ErrNotFound when
// the session was never pinned.
func (s *Store) Load(sessionID string) (*PinnedAnswer, error) {
	data, err := os.ReadFile(s.Path(sessionID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, sessionID)
	}
	if err != nil {
		return nil, err
	}
	var pa PinnedAnswer
	if err := json.Unmarshal(data, &pa); err != nil {
		return nil, fmt.Errorf("parse pin artifact %s: %w", s.Path(sessionID), err)
	}
	return &pa, nil
}

// Save atomically writes the artifact: a temp file in the same directory is
// flushed, synced and renamed over the target, so readers either see the old
// or the new artifact, never a partial one.
func (s *Store) Save(pa *PinnedAnswer) error {
	data, err := json.MarshalIndent(pa, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, ".pin-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpName, s.Path(pa.SessionID))
}

// Tracker observes one session's evolving answer and persists a new revision
// whenever the answer text changes; unchanged answers are left alone. The
// revision counter continues from the artifact already on disk, so restarting
// tailpin mid-run does not reset the answer's history.
type Tracker struct {
	store    *Store
	lastText string
	lastRev  int
}

// NewTracker loads the session's existing artifact, if any, to seed the
// revision counter.
func NewTracker(store *Store, sessionID string) *Tracker {
	t := &Tracker{store: store}
	if pa, err := store.Load(sessionID); err == nil {
		t.lastText, t.lastRev = pa.AnswerText, pa.Revision
	}
	return t
}

// Observe records the latest extracted answer. It assigns the next revision
// and saves only when the answer text changed, mutating pa so the caller can
// render the persisted revision and timestamp. A nil answer (the session has
// no extractable conclusion yet) never overwrites a stored artifact.
func (t *Tracker) Observe(pa *PinnedAnswer) error {
	if pa == nil {
		return nil
	}
	if pa.AnswerText == t.lastText {
		return nil
	}
	t.lastRev++
	pa.Revision = t.lastRev
	pa.UpdatedAt = time.Now().UTC()
	if err := t.store.Save(pa); err != nil {
		t.lastRev--
		return err
	}
	t.lastText = pa.AnswerText
	return nil
}

// Revision reports the last persisted revision for the session.
func (t *Tracker) Revision() int {
	return t.lastRev
}
