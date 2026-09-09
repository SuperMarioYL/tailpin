package pin

import (
	"strings"
	"testing"

	"github.com/SuperMarioYL/tailpin/internal/transcript"
)

const sampleFixture = "../transcript/testdata/sample.jsonl"

func loadFixtureSpans(t *testing.T) []transcript.TextSpan {
	t.Helper()
	sess, err := transcript.Load(sampleFixture)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return sess.Spans
}

func TestExtractPicksFinalSummaryWithProvenance(t *testing.T) {
	spans := loadFixtureSpans(t)
	pa := Extract("9f3c2a10-6d1b-4c2e-8f0a-3b7d2c9e1a44", spans)
	if pa == nil {
		t.Fatal("Extract returned nil for a session with a final summary")
	}
	if pa.SessionID != "9f3c2a10-6d1b-4c2e-8f0a-3b7d2c9e1a44" {
		t.Errorf("SessionID = %q", pa.SessionID)
	}
	// The final summary lives on fixture line 20, record uuid ...013.
	if len(pa.Claims) != 5 {
		t.Fatalf("claims = %d, want 5 (intro + 4 bullets): %+v", len(pa.Claims), pa.Claims)
	}
	wantKinds := []Kind{KindFix, KindFinding, KindFix, KindFinding, KindCaveat}
	wantLead := []string{"In summary", "Root cause", "Fix", "Verified", "Caveat"}
	for i, c := range pa.Claims {
		if c.Kind != wantKinds[i] {
			t.Errorf("claim %d kind = %q, want %q (text %q)", i, c.Kind, wantKinds[i], c.Text)
		}
		if lead := wantLead[i]; !strings.HasPrefix(strings.TrimSpace(c.Text), lead) {
			t.Errorf("claim %d text = %q, want prefix %q", i, c.Text, lead)
		}
		if c.Span.LineStart != 20 || c.Span.LineEnd != 20 {
			t.Errorf("claim %d span lines = %d-%d, want 20-20", i, c.Span.LineStart, c.Span.LineEnd)
		}
		if c.Span.MessageUUID != "a1b0c2d3-0000-4000-8000-000000000013" {
			t.Errorf("claim %d span uuid = %q", i, c.Span.MessageUUID)
		}
		if c.Span.Timestamp != "2026-09-08T09:16:40.003Z" {
			t.Errorf("claim %d span timestamp = %q", i, c.Span.Timestamp)
		}
		if c.Span.File != sampleFixture {
			t.Errorf("claim %d span file = %q", i, c.Span.File)
		}
	}
	if pa.Confidence != ConfidenceHigh {
		t.Errorf("confidence = %q, want high for a marker-rich final summary", pa.Confidence)
	}
	// The invariant the span viewer relies on: every claim is a substring of
	// the winning span's text, so the claim can be highlighted in place.
	winner := spans[len(spans)-1]
	for i, c := range pa.Claims {
		if !strings.Contains(winner.Text, c.Text) {
			t.Errorf("claim %d is not a substring of its span text", i)
		}
	}
	if pa.AnswerText != strings.Join(claimTexts(pa), "\n") {
		t.Error("AnswerText must be the claims joined by newlines")
	}
}

func TestExtractChineseConclusionWins(t *testing.T) {
	spans := []transcript.TextSpan{
		{Text: "Let me read the config first.", MessageUUID: "narration"},
		{Text: "总结：问题的根本原因在 internal/parser/parser.go 的循环边界。修复方法是收紧判断条件。注意：旧测试依赖这个行为。", MessageUUID: "zh-answer"},
	}
	pa := Extract("s1", spans)
	if pa == nil {
		t.Fatal("Extract returned nil")
	}
	if got := pa.Claims[0].Span.MessageUUID; got != "zh-answer" {
		t.Fatalf("winning span = %q, want the Chinese conclusion", got)
	}
	if len(pa.Claims) != 3 {
		t.Fatalf("claims = %d, want 3 CJK sentences: %+v", len(pa.Claims), claimTexts(pa))
	}
	wantKinds := []Kind{KindFinding, KindFix, KindCaveat}
	for i, want := range wantKinds {
		if pa.Claims[i].Kind != want {
			t.Errorf("claim %d kind = %q, want %q", i, pa.Claims[i].Kind, want)
		}
	}
	if pa.Confidence == ConfidenceLow {
		t.Error("marker-rich Chinese conclusion should not be low confidence")
	}
}

func TestExtractDeprioritizesPermissionAsks(t *testing.T) {
	spans := []transcript.TextSpan{
		{Text: "I'll check the parser implementation now.", MessageUUID: "narration"},
		{Text: "Should I also refactor the config loader? It would touch 3 more files.", MessageUUID: "ask"},
	}
	pa := Extract("s1", spans)
	if pa == nil {
		t.Fatal("Extract returned nil")
	}
	if got := pa.Claims[0].Span.MessageUUID; got != "narration" {
		t.Errorf("winning span = %q, want the narration over the permission ask", got)
	}
	if pa.Confidence != ConfidenceLow {
		t.Errorf("confidence = %q, want low", pa.Confidence)
	}
}

func TestExtractEmpty(t *testing.T) {
	if pa := Extract("s1", nil); pa != nil {
		t.Errorf("Extract(nil spans) = %+v, want nil", pa)
	}
	if pa := Extract("s1", []transcript.TextSpan{{Text: "   ", MessageUUID: "blank"}}); pa != nil {
		t.Errorf("Extract(whitespace span) = %+v, want nil", pa)
	}
}

func TestSplitSentencesKeepsCodeBlocksWhole(t *testing.T) {
	text := "The fix is twofold. First, the loop:\n```\nfor i := 0; i < n; i++ {\n}\n```\nThat is all."
	claims := splitSentences(text)
	want := []string{
		"The fix is twofold.",
		"First, the loop:",
		"```\nfor i := 0; i < n; i++ {\n}\n```",
		"That is all.",
	}
	if len(claims) != len(want) {
		t.Fatalf("claims = %#v, want %#v", claims, want)
	}
	for i := range want {
		if claims[i] != want[i] {
			t.Errorf("claim %d = %q, want %q", i, claims[i], want[i])
		}
	}
}

func TestSplitBulletedGroupsContinuations(t *testing.T) {
	text := "Intro line:\n- first bullet\n  continued indent\n- second bullet\n* third style\n1. numbered too\n"
	claims := splitClaims(text)
	want := []string{
		"Intro line:",
		"- first bullet\n  continued indent",
		"- second bullet",
		"* third style",
		"1. numbered too",
	}
	if len(claims) != len(want) {
		t.Fatalf("claims = %#v, want %#v", claims, want)
	}
	for i := range want {
		if claims[i] != want[i] {
			t.Errorf("claim %d = %q, want %q", i, claims[i], want[i])
		}
	}
}

func TestClassifyClaim(t *testing.T) {
	cases := []struct {
		text string
		want Kind
	}{
		{"Note: the cache is shared.", KindCaveat},
		{"Caveat: pending refactor.", KindCaveat},
		{"However, the API rate limit applies.", KindCaveat},
		{"注意：旧测试依赖这个行为。", KindCaveat},
		{"Next, I will add tests.", KindPlan},
		{"接下来会修补边界判断。", KindPlan},
		{"Fixed the loop boundary at parser.go:42.", KindFix},
		{"修复方法是收紧判断条件。", KindFix},
		{"The parser double-counts newlines.", KindFinding},
		{"Root cause: off-by-one.", KindFinding},
	}
	for _, c := range cases {
		if got := classifyClaim(c.text); got != c.want {
			t.Errorf("classifyClaim(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestExtractCapsClaims(t *testing.T) {
	var b strings.Builder
	b.WriteString("Summary of findings:\n")
	for i := 0; i < 40; i++ {
		b.WriteString("- finding number ")
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString(" keeps this line long enough to matter\n")
	}
	pa := Extract("s1", []transcript.TextSpan{{Text: b.String(), MessageUUID: "many"}})
	if pa == nil {
		t.Fatal("Extract returned nil")
	}
	if len(pa.Claims) != maxClaims {
		t.Errorf("claims = %d, want cap %d", len(pa.Claims), maxClaims)
	}
}

func claimTexts(pa *PinnedAnswer) []string {
	out := make([]string, len(pa.Claims))
	for i, c := range pa.Claims {
		out[i] = c.Text
	}
	return out
}
