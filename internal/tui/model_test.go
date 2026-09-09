package tui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SuperMarioYL/tailpin/internal/pin"
	"github.com/SuperMarioYL/tailpin/internal/transcript"
)

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// update applies one message and returns the concrete model.
func update(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

// loadedFixtureModel returns a model that has ingested the sample session at
// a fixed terminal size.
func loadedFixtureModel(t *testing.T) Model {
	t.Helper()
	sess, err := transcript.Load(sampleFixture)
	if err != nil {
		t.Fatal(err)
	}
	store, err := pin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(sampleFixture, nil, store)
	m.width, m.height = 100, 40
	m = update(m, sessionLoadedMsg{session: sess})
	return m
}

func TestModelRendersPinnedAnswerWithProvenance(t *testing.T) {
	m := loadedFixtureModel(t)
	view := m.View()
	for _, want := range []string{
		"tailpin", "9f3c2a10", "rev 1", "high",
		"In summary, the flaky parser test is fixed:",
		"Root cause: countTokens",
		"[caveat]",
		"L20 · a1b0c2d3",
		"j/k claims · enter span · q quit",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "waiting for the agent") {
		t.Error("view should show the fixture answer, not the waiting placeholder")
	}
}

func TestModelNavigation(t *testing.T) {
	m := loadedFixtureModel(t)
	if m.selected != 0 {
		t.Fatalf("initial selected = %d, want 0", m.selected)
	}
	m = update(m, key("j"))
	m = update(m, key("j"))
	if m.selected != 2 {
		t.Fatalf("after j j, selected = %d, want 2", m.selected)
	}
	m = update(m, key("k"))
	if m.selected != 1 {
		t.Fatalf("after k, selected = %d, want 1", m.selected)
	}
	// Selection clamps at both ends.
	for i := 0; i < 10; i++ {
		m = update(m, key("k"))
	}
	if m.selected != 0 {
		t.Fatalf("selected should clamp at 0, got %d", m.selected)
	}
	for i := 0; i < 10; i++ {
		m = update(m, key("j"))
	}
	if want := len(m.claims()) - 1; m.selected != want {
		t.Fatalf("selected should clamp at %d, got %d", want, m.selected)
	}
	// The selected claim is visibly marked.
	view := m.View()
	if !strings.Contains(view, "❯") {
		t.Errorf("selected claim should carry a marker:\n%s", view)
	}
}

func TestModelSpanViewer(t *testing.T) {
	m := loadedFixtureModel(t)
	m = update(m, key("j")) // select the Root cause claim
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.viewerOpen {
		t.Fatal("enter should open the span viewer")
	}
	view := m.View()
	for _, want := range []string{"── span", "claim: Root cause", "L20 · a1b0c2d3"} {
		if !strings.Contains(view, want) {
			t.Errorf("viewer missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "j/k scroll · esc back · q quit") {
		t.Errorf("viewer footer should switch key hints:\n%s", view)
	}

	// While the viewer is open, j/k scroll it instead of moving the cursor.
	before := m.selected
	m = update(m, key("j"))
	if m.selected != before || m.viewerScroll != 1 {
		t.Errorf("j in viewer should scroll (selected=%d scroll=%d)", m.selected, m.viewerScroll)
	}
	m = update(m, key("k"))
	if m.viewerScroll != 0 {
		t.Errorf("k in viewer should scroll back, got %d", m.viewerScroll)
	}

	// The view is clamped to the terminal height.
	if n := len(strings.Split(view, "\n")); n > m.height {
		t.Errorf("view is %d lines, exceeds terminal height %d", n, m.height)
	}

	m = update(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.viewerOpen {
		t.Error("esc should close the span viewer")
	}
	if strings.Contains(m.View(), "── span") {
		t.Error("closed viewer should not render a span pane")
	}
}

func TestModelQuitOnQ(t *testing.T) {
	m := loadedFixtureModel(t)
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Error("q should quit the program")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("ctrl+c should quit the program")
	}
}

// writeSession writes a minimal Claude-Code-style session file with one
// assistant text record per text argument.
func writeSession(t *testing.T, path string, texts ...string) {
	t.Helper()
	var b strings.Builder
	for i, text := range texts {
		record := `{"type":"assistant","uuid":"u-` + strconv.Itoa(i) +
			`","sessionId":"synthetic","timestamp":"2026-09-08T09:00:0` + strconv.Itoa(i) +
			`.000Z","message":{"role":"assistant","content":[{"type":"text","text":` +
			jsonString(text) + `}]}}`
		b.WriteString(record + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// jsonString encodes s exactly the way a JSONL record would embed it.
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestModelUpdatesAnswerAndRevisionOnAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "synthetic.jsonl")
	writeSession(t, path, "Working on it.")

	store, err := pin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(path, nil, store)
	m.width, m.height = 100, 40

	sess1, err := transcript.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	m = update(m, sessionLoadedMsg{session: sess1})
	if m.answer == nil || m.answer.AnswerText != "Working on it." {
		t.Fatalf("first answer = %+v", m.answer)
	}
	if m.tracker.Revision() != 1 {
		t.Fatalf("first revision = %d, want 1", m.tracker.Revision())
	}

	// The harness appends a stronger conclusion; the watcher would deliver
	// the re-parsed session as the next sessionLoadedMsg.
	writeSession(t, path, "Working on it.",
		"In summary: the flaky test is fixed in internal/parser/parser.go and verified with go test ./...")
	sess2, err := transcript.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	m = update(m, sessionLoadedMsg{session: sess2})
	if m.answer == nil || !strings.Contains(m.answer.AnswerText, "In summary") {
		t.Fatalf("updated answer = %+v", m.answer)
	}
	if m.tracker.Revision() != 2 {
		t.Fatalf("revision after change = %d, want 2", m.tracker.Revision())
	}
	if view := m.View(); !strings.Contains(view, "rev 2") {
		t.Errorf("view should show rev 2:\n%s", view)
	}

	// Re-delivering the same session must not bump the revision again.
	m = update(m, sessionLoadedMsg{session: sess2})
	if m.tracker.Revision() != 2 {
		t.Fatalf("unchanged answer bumped revision to %d", m.tracker.Revision())
	}
}

func TestModelViewRendersAtDefaultSizeWithoutWindowSizeMsg(t *testing.T) {
	// Piped output never delivers a WindowSizeMsg; the view must still render
	// the answer at the default size instead of a loading placeholder.
	sess, err := transcript.Load(sampleFixture)
	if err != nil {
		t.Fatal(err)
	}
	store, err := pin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(sampleFixture, nil, store)
	m = update(m, sessionLoadedMsg{session: sess})
	view := m.View()
	if strings.Contains(view, "loading…") {
		t.Errorf("view without window size should render, not stall:\n%s", view)
	}
	if !strings.Contains(view, "In summary, the flaky parser test is fixed:") {
		t.Errorf("view at default size lost the answer:\n%s", view)
	}
	if n := len(strings.Split(view, "\n")); n > defaultHeight {
		t.Errorf("view is %d lines, exceeds default height %d", n, defaultHeight)
	}
}

func TestModelNoAnswerWaits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	sess, err := transcript.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	m := New(path, nil, nil)
	m.width, m.height = 80, 24
	m = update(m, sessionLoadedMsg{session: sess})
	if m.answer != nil {
		t.Fatal("empty session must not produce an answer")
	}
	if view := m.View(); !strings.Contains(view, "waiting for the agent's first conclusion") {
		t.Errorf("view should show the waiting placeholder:\n%s", view)
	}
}

func TestModelLoadErrorShowsStatus(t *testing.T) {
	m := New("/nonexistent/session.jsonl", nil, nil)
	m.width, m.height = 80, 24
	m = update(m, sessionLoadedMsg{err: errors.New("boom")})
	if !strings.Contains(m.View(), "boom") {
		t.Errorf("load error should surface in the header:\n%s", m.View())
	}
}

func TestWindowBlocksKeepsSelectionVisible(t *testing.T) {
	blocks := [][]string{{"a"}, {"b", "b"}, {"c", "c", "c"}}
	if got := windowBlocks(blocks, 0, 6); len(got) != 3 {
		t.Errorf("everything fits: got %d blocks, want 3", len(got))
	}
	got := windowBlocks(blocks, 2, 3)
	if len(got) != 1 || got[0][0] != "c" {
		t.Errorf("window for last block = %v, want just the 3-line block", got)
	}
	got = windowBlocks(blocks, 0, 3)
	if len(got) != 2 || len(got[0])+len(got[1]) != 3 {
		t.Errorf("window for first block = %v, want blocks 0+1 (3 lines)", got)
	}
	if got := windowBlocks(blocks, 0, 0); got != nil {
		t.Errorf("zero-height window should be empty, got %v", got)
	}
}

func TestTruncateLines(t *testing.T) {
	if got := truncateLines("a\nb\nc\nd", 2); got != "a\nb" {
		t.Errorf("truncateLines = %q", got)
	}
	if got := truncateLines("a\nb", 5); got != "a\nb" {
		t.Errorf("short input should pass through, got %q", got)
	}
	if got := truncateLines("a\nb", 0); got != "" {
		t.Errorf("zero height should be empty, got %q", got)
	}
}
