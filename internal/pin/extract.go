package pin

import (
	"math"
	"regexp"
	"strings"

	"github.com/SuperMarioYL/tailpin/internal/transcript"
)

// maxClaims keeps the pinned answer a short conclusion (claims joined stay
// within roughly 40 lines) instead of a second transcript.
const maxClaims = 30

// Scoring weights. Every component is deterministic and offline: recency,
// conclusion markers (English and Chinese), code blocks and file-path
// mentions raise a span; permission asks, questions and
// narrating-what-I'm-about-to-do lower it.
const (
	recencyWeight     = 3.0
	substanceWeight   = 2.0 // saturates at ~200 runes of text
	markerBonus       = 1.5 // per distinct conclusion marker
	markerCap         = 4.5
	codeBlockBonus    = 1.0 // per fenced code block
	codeBlockCap      = 2.0
	pathBonus         = 0.5 // per distinct file-path mention
	pathCap           = 1.5
	permissionPenalty = 6.0
	questionPenalty   = 0.75 // per question mark
	questionCap       = 2.25
	narrationPenalty  = 2.0
)

// Confidence thresholds on the winning span's score.
const (
	highConfidenceAt = 5.0
	medConfidenceAt  = 2.5
)

var conclusionMarkers = []string{
	// English
	"in summary", "to summarize", "summary:", "conclusion", "in short",
	"tldr", "tl;dr", "the answer", "answer:", "final answer", "root cause",
	"next steps", "the fix", "fixed", "completed", "done", "verified",
	"recommend", "result:",
	// Chinese
	"总结", "结论", "答案是", "最终", "要点", "修复", "已修复", "根本原因",
	"建议", "方案", "完成", "已验证",
}

var permissionAsks = []string{
	"should i", "shall i", "may i", "do you want", "would you like",
	"can i proceed", "do you approve", "需要我", "要不要", "是否需要", "可以吗",
}

var narrationStarts = []string{
	"i'll ", "i will ", "let me ", "i'm going to ", "i am going to ",
	"让我", "我来", "我先",
}

var (
	bulletRe = regexp.MustCompile(`^\s{0,3}(?:[-*+]|\d{1,3}[.)])\s+`)
	pathRe   = regexp.MustCompile(`(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+`)
)

// Extract derives the current pinned answer from a session's assistant text
// spans. The best-scoring span becomes the answer and is split into claims;
// every claim carries that span's provenance, so a wrong heuristic is
// visible in one keystroke. Returns nil when there is no answer yet.
func Extract(sessionID string, spans []transcript.TextSpan) *PinnedAnswer {
	if len(spans) == 0 {
		return nil
	}
	bestIdx, bestScore := 0, math.Inf(-1)
	for i := range spans {
		s := scoreSpan(spans[i].Text, recency(i, len(spans)))
		if s > bestScore {
			bestIdx, bestScore = i, s
		}
	}
	span := spans[bestIdx]
	claims := splitClaims(span.Text)
	if len(claims) == 0 {
		return nil
	}
	if len(claims) > maxClaims {
		claims = claims[:maxClaims]
	}
	provenance := Span{
		File:        span.File,
		LineStart:   span.LineStart,
		LineEnd:     span.LineEnd,
		ByteStart:   span.ByteStart,
		ByteEnd:     span.ByteEnd,
		MessageUUID: span.MessageUUID,
		Timestamp:   span.Timestamp,
	}
	var texts []string
	for _, text := range claims {
		if trimmed := trimBulletMarker(text); trimmed != "" {
			texts = append(texts, trimmed)
		}
	}
	pa := &PinnedAnswer{
		SessionID:  sessionID,
		AnswerText: strings.Join(texts, "\n"),
		Confidence: confidenceFor(bestScore),
	}
	for _, text := range texts {
		pa.Claims = append(pa.Claims, Claim{Text: text, Kind: classifyClaim(text), Span: provenance})
	}
	return pa
}

// trimBulletMarker strips a leading bullet or list marker so each claim reads
// as a standalone sentence; the raw span keeps the original formatting, and
// the trimmed claim remains a substring of the span text.
func trimBulletMarker(text string) string {
	if loc := bulletRe.FindStringIndex(text); loc != nil && loc[0] == 0 {
		return strings.TrimSpace(text[loc[1]:])
	}
	return text
}

func recency(i, n int) float64 {
	return float64(i+1) / float64(n)
}

func confidenceFor(score float64) Confidence {
	switch {
	case score >= highConfidenceAt:
		return ConfidenceHigh
	case score >= medConfidenceAt:
		return ConfidenceMed
	default:
		return ConfidenceLow
	}
}

// scoreSpan applies the deterministic ranking described in the plan.
func scoreSpan(text string, rec float64) float64 {
	lower := strings.ToLower(text)
	runes := len([]rune(strings.TrimSpace(text)))

	score := rec * recencyWeight
	score += min2(substanceWeight*float64(runes)/200.0, substanceWeight)

	score += capped(float64(countDistinctMarkers(lower)), markerBonus, markerCap)
	score += capped(float64(codeBlocks(text)), codeBlockBonus, codeBlockCap)
	score += capped(float64(distinctPaths(text)), pathBonus, pathCap)

	for _, ask := range permissionAsks {
		if strings.Contains(lower, ask) {
			score -= permissionPenalty
			break
		}
	}
	score -= capped(float64(strings.Count(text, "?")+strings.Count(text, "？")), questionPenalty, questionCap)

	trimmed := strings.TrimSpace(lower)
	for _, start := range narrationStarts {
		if strings.HasPrefix(trimmed, start) {
			score -= narrationPenalty
			break
		}
	}
	return score
}

func countDistinctMarkers(lower string) int {
	n := 0
	for _, m := range conclusionMarkers {
		if strings.Contains(lower, m) {
			n++
		}
	}
	return n
}

func codeBlocks(text string) int {
	return strings.Count(text, "```") / 2
}

func distinctPaths(text string) int {
	seen := map[string]bool{}
	for _, p := range pathRe.FindAllString(text, -1) {
		seen[p] = true
	}
	return len(seen)
}

func capped(n float64, per, cap float64) float64 {
	v := float64(n) * per
	if v > cap {
		return cap
	}
	return v
}

func min2(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// splitClaims splits a span's text into claims: one sentence or bullet each.
// Bullet lists split per bullet (continuation lines included); prose splits
// on sentence boundaries and newlines; fenced code blocks stay whole. Each
// claim remains a substring of the span text so the viewer can highlight it
// in place.
func splitClaims(text string) []string {
	lines := strings.Split(text, "\n")
	hasBullet := false
	for _, line := range lines {
		if bulletRe.MatchString(line) {
			hasBullet = true
			break
		}
	}
	if hasBullet {
		return splitBulleted(lines)
	}
	return splitSentences(text)
}

// splitBulleted groups a bullet list: lines before the first bullet form one
// intro claim, each bullet plus its indented continuations forms a claim.
// Fence-aware so code inside a bullet is never mistaken for new bullets.
func splitBulleted(lines []string) []string {
	var claims []string
	var current []string
	inFence := false
	flush := func() {
		if len(current) > 0 {
			if s := strings.TrimSpace(strings.Join(current, "\n")); s != "" {
				claims = append(claims, s)
			}
			current = nil
		}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			current = append(current, line)
			continue
		}
		if !inFence && bulletRe.MatchString(line) {
			flush()
			current = []string{line}
			continue
		}
		if strings.TrimSpace(line) == "" && !inFence {
			// Blank lines outside fences separate groups but stay out of
			// the claim text.
			continue
		}
		current = append(current, line)
	}
	flush()
	return claims
}

// splitSentences cuts prose into claims at sentence enders (English and
// Chinese) and newlines, keeping fenced code blocks whole.
func splitSentences(text string) []string {
	var claims []string
	var cur strings.Builder
	runes := []rune(text)
	inFence := false
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			claims = append(claims, s)
		}
		cur.Reset()
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '`' && i+2 < len(runes) && runes[i+1] == '`' && runes[i+2] == '`' {
			inFence = !inFence
			cur.WriteString("```")
			i += 2
			continue
		}
		cur.WriteRune(r)
		if inFence {
			continue
		}
		if r == '\n' {
			flush()
			continue
		}
		if isSentenceEnd(r) && (isCJKSentenceEnder(r) || i+1 >= len(runes) || isSpace(runes[i+1])) {
			flush()
		}
	}
	flush()
	return claims
}

func isSentenceEnd(r rune) bool {
	switch r {
	case '.', '!', '?', '。', '！', '？':
		return true
	}
	return false
}

// isCJKSentenceEnder reports whether r is a CJK sentence ender. Chinese
// sentences follow each other without whitespace, so these always cut.
func isCJKSentenceEnder(r rune) bool {
	return r == '。' || r == '！' || r == '？'
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// classifyClaim labels a claim deterministically: caveats first (the reader
// must not miss them), then plans, then fixes, everything else is a finding.
func classifyClaim(text string) Kind {
	head := strings.ToLower(text)
	if len(head) > 40 {
		head = head[:40]
	}
	if hasStarter(head, "note", "caveat", "warning", "caution", "however", "but", "注意", "但是", "不过", "风险", "提醒") {
		return KindCaveat
	}
	if hasStarter(head, "next", "then", "plan", "will", "step", "todo", "接下来", "计划", "将要", "准备", "待办") {
		return KindPlan
	}
	if strings.Contains(head, "fix") || strings.Contains(head, "bug") || strings.Contains(head, "patch") ||
		strings.Contains(text, "```") || strings.Contains(head, "修复") || strings.Contains(head, "解决") {
		return KindFix
	}
	return KindFinding
}

// hasStarter reports whether head starts with one of the given words,
// followed by a non-letter boundary so "note" does not match "noteworthy".
func hasStarter(head string, starters ...string) bool {
	for _, s := range starters {
		if !strings.HasPrefix(head, s) {
			continue
		}
		rest := head[len(s):]
		if rest == "" || !isASCIILetter(rest[0]) {
			return true
		}
	}
	return false
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
