package pin

import (
	"strings"
	"testing"
	"time"
)

func answerWithClaims(sessionID string, texts ...string) *PinnedAnswer {
	pa := &PinnedAnswer{
		SessionID:  sessionID,
		AnswerText: strings.Join(texts, "\n"),
		Confidence: ConfidenceMed,
		UpdatedAt:  time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC),
	}
	for _, t := range texts {
		pa.Claims = append(pa.Claims, Claim{Text: t, Kind: KindFinding})
	}
	return pa
}

func TestDiffIdentifiesAddedRemovedChangedUnchanged(t *testing.T) {
	a := answerWithClaims("run-a",
		"Root cause: the loop boundary is off by one.",
		"The fix lands in parser.go.",
		"Verified: go test passes.",
	)
	b := answerWithClaims("run-b",
		"Root cause: the loop boundary is off by one.",    // unchanged
		"Verified: go test passes with -race too.",        // changed from "Verified: go test passes."
		"Also added a regression case to parser_test.go.", // added
		// "The fix lands in parser.go." was removed
	)
	res := Diff(a, b)
	if res.SessionA != "run-a" || res.SessionB != "run-b" {
		t.Errorf("session ids = %q -> %q", res.SessionA, res.SessionB)
	}
	if res.Unchanged != 1 {
		t.Errorf("Unchanged = %d, want 1", res.Unchanged)
	}
	if len(res.Changed) != 1 || res.Changed[0].From.Text != "Verified: go test passes." ||
		res.Changed[0].To.Text != "Verified: go test passes with -race too." {
		t.Errorf("Changed = %+v, want the two Verified sentences paired", res.Changed)
	}
	if len(res.Added) != 1 || res.Added[0].Text != "Also added a regression case to parser_test.go." {
		t.Errorf("Added = %+v", res.Added)
	}
	if len(res.Removed) != 1 || res.Removed[0].Text != "The fix lands in parser.go." {
		t.Errorf("Removed = %+v", res.Removed)
	}
}

func TestDiffNormalizesBulletMarkers(t *testing.T) {
	// The same claim written as a bullet in one run and as prose in the other
	// is unchanged, not a remove plus an add.
	a := answerWithClaims("a", "- Root cause: the loop boundary is off by one.")
	b := answerWithClaims("b", "Root cause: the loop boundary is off by one.")
	res := Diff(a, b)
	if res.Unchanged != 1 || len(res.Changed)+len(res.Added)+len(res.Removed) != 0 {
		t.Errorf("bullet vs prose should be unchanged, got %+v", res)
	}
}

func TestDiffPairsRevisedChineseClaims(t *testing.T) {
	a := answerWithClaims("a", "总结：问题的根本原因在循环边界。")
	b := answerWithClaims("b", "总结：问题的根本原因在循环边界判断。")
	res := Diff(a, b)
	if len(res.Changed) != 1 {
		t.Fatalf("Changed = %d, want 1 (a revised sentence, not remove+add): %+v", len(res.Changed), res)
	}
	if res.Changed[0].To.Text != "总结：问题的根本原因在循环边界判断。" {
		t.Errorf("changed To = %q", res.Changed[0].To.Text)
	}
}

func TestDiffNilAnswers(t *testing.T) {
	a := answerWithClaims("a", "one claim")
	res := Diff(a, nil)
	if len(res.Removed) != 1 || res.Unchanged != 0 {
		t.Errorf("Diff(answer, nil) = %+v, want the single claim removed", res)
	}
	res = Diff(nil, a)
	if len(res.Added) != 1 {
		t.Errorf("Diff(nil, answer) = %+v, want the single claim added", res)
	}
	if res.SessionA != "(no pinned answer)" {
		t.Errorf("SessionA = %q, want the no-answer placeholder", res.SessionA)
	}
}

func TestSimilarityBounds(t *testing.T) {
	if s := similarity("", "anything"); s != 0 {
		t.Errorf("similarity(empty, x) = %v, want 0", s)
	}
	if s := similarity("identical", "identical"); s != 1 {
		t.Errorf("similarity(x, x) = %v, want 1", s)
	}
	// Unrelated claims must stay below the changed threshold.
	if s := similarity("the parser fails on newline", "buy milk and eggs tomorrow"); s >= changedAt {
		t.Errorf("unrelated claims similarity = %v, want < %v", s, changedAt)
	}
}

func TestFormatDiffPlainAndColored(t *testing.T) {
	a := answerWithClaims("run-a",
		"The fix lands in parser.go.",
		"Verified: go test passes.",
	)
	b := answerWithClaims("run-b",
		"Verified: go test passes with -race too.",
		"Also added a regression case to parser_test.go.",
	)
	res := Diff(a, b)

	plain := FormatDiff(res, false)
	for _, want := range []string{"run-a -> run-b", "~ changed", "was:", "+ added", "- removed", "0 claims unchanged"} {
		if !strings.Contains(plain, want) {
			t.Errorf("plain diff missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "\x1b[") {
		t.Error("plain diff must not contain ANSI escapes")
	}

	colored := FormatDiff(res, true)
	if !strings.Contains(colored, "\x1b[") {
		t.Error("colored diff should contain ANSI escapes")
	}
}

func TestFormatDiffNoChanges(t *testing.T) {
	res := Diff(nil, nil)
	out := FormatDiff(res, false)
	if !strings.Contains(out, "no claim-level changes") {
		t.Errorf("empty diff output = %q", out)
	}
}
