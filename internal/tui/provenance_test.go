package tui

import (
	"strings"
	"testing"

	"github.com/SuperMarioYL/tailpin/internal/pin"
	"github.com/SuperMarioYL/tailpin/internal/transcript"
)

const sampleFixture = "../transcript/testdata/sample.jsonl"

func TestSpanWindow(t *testing.T) {
	cases := []struct {
		total, start, end, context int
		wantStart, wantEnd         int
	}{
		{20, 20, 20, 8, 12, 20},  // tail span: clamp at file end
		{20, 3, 3, 8, 1, 11},     // head span: clamp at file start
		{20, 10, 12, 100, 1, 20}, // context larger than file: whole file
		{20, 12, 10, 1, 9, 13},   // swapped line order is tolerated
		{0, 1, 1, 5, 0, 0},       // empty transcript
	}
	for _, c := range cases {
		gotStart, gotEnd := spanWindow(c.total, c.start, c.end, c.context)
		if gotStart != c.wantStart || gotEnd != c.wantEnd {
			t.Errorf("spanWindow(%d, %d, %d, %d) = %d,%d want %d,%d",
				c.total, c.start, c.end, c.context, gotStart, gotEnd, c.wantStart, c.wantEnd)
		}
	}
}

func TestProvenanceLabel(t *testing.T) {
	single := pin.Span{
		LineStart:   20,
		LineEnd:     20,
		MessageUUID: "a1b0c2d3-0000-4000-8000-000000000013",
		Timestamp:   "2026-09-08T09:16:40.003Z",
	}
	if got, want := provenanceLabel(single), "L20 · a1b0c2d3 · 2026-09-08T09:16:40.003Z"; got != want {
		t.Errorf("provenanceLabel = %q, want %q", got, want)
	}
	multi := single
	multi.LineEnd = 22
	if got := provenanceLabel(multi); !strings.Contains(got, "L20-L22") {
		t.Errorf("multi-line label = %q, want L20-L22 range", got)
	}
}

func TestClaimAnchor(t *testing.T) {
	// The anchor is the claim's first line capped at anchorRunes runes.
	want := truncateRunes("In summary, the flaky parser test is fixed:", anchorRunes)
	if got := claimAnchor("In summary, the flaky parser test is fixed:\nsecond line"); got != want {
		t.Errorf("claimAnchor = %q, want %q", got, want)
	}
	if got := claimAnchor("  padded  "); got != "padded" {
		t.Errorf("claimAnchor = %q, want trimmed text", got)
	}
}

func TestFrameLine(t *testing.T) {
	long := strings.Repeat("x", 30) + "ANCHOR-HERE" + strings.Repeat("y", 200)

	short := frameLine("short line", "anchor", 40)
	if short != "short line" {
		t.Errorf("short line should pass through, got %q", short)
	}

	framed := frameLine(long, "ANCHOR-HERE", 40)
	if !strings.Contains(framed, "ANCHOR-HERE") {
		t.Errorf("framed line lost the anchor: %q", framed)
	}
	if n := len([]rune(framed)); n > 40 {
		t.Errorf("framed line = %d runes, want <= 40", n)
	}

	head := frameLine(long, "", 40)
	if strings.HasPrefix(head, "…") || !strings.HasSuffix(head, "…") {
		t.Errorf("anchorless frame should show the record head: %q", head)
	}
	if !strings.HasPrefix(head, "xxx") {
		t.Errorf("anchorless frame should start at the line head: %q", head)
	}
}

func TestViewerContentShowsRawRecordsAroundSpan(t *testing.T) {
	sess, err := transcript.Load(sampleFixture)
	if err != nil {
		t.Fatal(err)
	}
	pa := pin.Extract(sess.SessionID, sess.Spans)
	if pa == nil || len(pa.Claims) == 0 {
		t.Fatal("fixture must yield a pinned answer")
	}
	lines := viewerContent(sess, pa.Claims[0], 100)
	joined := strings.Join(lines, "\n")

	// The claim, its provenance, and the raw records are all present.
	if !strings.Contains(joined, "claim: ") || !strings.Contains(joined, pa.Claims[0].Text) {
		t.Errorf("viewer lost the claim text:\n%s", joined)
	}
	if !strings.Contains(joined, "L20") {
		t.Errorf("viewer lost the provenance label:\n%s", joined)
	}
	if !strings.Contains(joined, "In summary, the flaky parser test is fixe") {
		t.Errorf("producing record should show the claim anchor:\n%s", joined)
	}
	// spanWindow(20, 20, 20, 8) shows lines 12..20: context plus span.
	if n := len(lines); n != 3+9 { // claim + label + blank, then 9 records
		t.Errorf("viewer has %d lines, want 12 (claim+label+blank+9 records)", n)
	}
	if !strings.Contains(joined, "19 ") || !strings.Contains(joined, "last-prompt") {
		t.Errorf("context record before the span should be visible:\n%s", joined)
	}
}
