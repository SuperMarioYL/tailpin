package pin

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// changedAt is the similarity threshold above which two claims are the same
// claim, revised, rather than a removal plus an addition.
const changedAt = 0.5

// ClaimChange pairs an old claim with its revised version.
type ClaimChange struct {
	From Claim
	To   Claim
}

// DiffResult is the claim-level comparison of two pinned answers: what the
// agent concluded differently across two runs of the same task.
type DiffResult struct {
	SessionA   string // older session
	SessionB   string // newer session
	UpdatedAtA time.Time
	UpdatedAtB time.Time
	Added      []Claim       // claims only in the newer answer
	Removed    []Claim       // claims only in the older answer
	Changed    []ClaimChange // same claim, different text
	Unchanged  int
}

// Diff compares two pinned answers claim by claim. A nil answer counts as an
// empty answer, so diffing a run against "never pinned" lists every claim as
// added or removed rather than failing.
func Diff(a, b *PinnedAnswer) *DiffResult {
	res := &DiffResult{SessionA: sessionIDOf(a), SessionB: sessionIDOf(b)}
	if a != nil {
		res.UpdatedAtA = a.UpdatedAt
	}
	if b != nil {
		res.UpdatedAtB = b.UpdatedAt
	}

	// Pass 1: identical claims (after bullet-marker and whitespace
	// normalization) are unchanged.
	usedA := make([]bool, len(claimsOf(a)))
	usedB := make([]bool, len(claimsOf(b)))
	for bi, bc := range claimsOf(b) {
		for ai, ac := range claimsOf(a) {
			if usedA[ai] || normalizeClaim(ac.Text) != normalizeClaim(bc.Text) {
				continue
			}
			usedA[ai], usedB[bi] = true, true
			res.Unchanged++
			break
		}
	}

	// Pass 2: pair the leftovers greedily by character-bigram similarity
	// (works for English and Chinese alike); similar enough means changed.
	type pair struct {
		ai, bi int
		score  float64
	}
	var candidates []pair
	for ai, ac := range claimsOf(a) {
		if usedA[ai] {
			continue
		}
		for bi, bc := range claimsOf(b) {
			if usedB[bi] {
				continue
			}
			if s := similarity(normalizeClaim(ac.Text), normalizeClaim(bc.Text)); s >= changedAt {
				candidates = append(candidates, pair{ai, bi, s})
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].ai != candidates[j].ai {
			return candidates[i].ai < candidates[j].ai
		}
		return candidates[i].bi < candidates[j].bi
	})
	for _, p := range candidates {
		if usedA[p.ai] || usedB[p.bi] {
			continue
		}
		usedA[p.ai], usedB[p.bi] = true, true
		res.Changed = append(res.Changed, ClaimChange{From: claimsOf(a)[p.ai], To: claimsOf(b)[p.bi]})
	}

	for ai, ac := range claimsOf(a) {
		if !usedA[ai] {
			res.Removed = append(res.Removed, ac)
		}
	}
	for bi, bc := range claimsOf(b) {
		if !usedB[bi] {
			res.Added = append(res.Added, bc)
		}
	}
	return res
}

func sessionIDOf(pa *PinnedAnswer) string {
	if pa == nil {
		return "(no pinned answer)"
	}
	return pa.SessionID
}

func claimsOf(pa *PinnedAnswer) []Claim {
	if pa == nil {
		return nil
	}
	return pa.Claims
}

// normalizeClaim strips bullet markers and redundant whitespace so the same
// sentence written as a bullet or as prose compares equal.
func normalizeClaim(text string) string {
	s := strings.TrimSpace(text)
	for {
		if loc := bulletRe.FindStringIndex(s); loc != nil && loc[0] == 0 {
			s = strings.TrimSpace(s[loc[1]:])
			continue
		}
		break
	}
	s = strings.ToLower(s)
	return strings.Join(strings.Fields(s), " ")
}

// similarity is the character-bigram Jaccard overlap of two normalized
// claims, in [0, 1]. Zero when either side has no bigram.
func similarity(a, b string) float64 {
	ga, gb := bigrams(a), bigrams(b)
	if len(ga) == 0 || len(gb) == 0 {
		return 0
	}
	inter, totalA, totalB := 0, 0, 0
	for k, n := range ga {
		totalA += n
		if m, ok := gb[k]; ok {
			inter += min(n, m)
		}
	}
	for _, n := range gb {
		totalB += n
	}
	union := totalA + totalB - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func bigrams(s string) map[string]int {
	runes := []rune(s)
	if len(runes) < 2 {
		if len(runes) == 1 {
			return map[string]int{string(runes): 1}
		}
		return nil
	}
	out := make(map[string]int, len(runes))
	for i := 0; i+1 < len(runes); i++ {
		out[string(runes[i:i+2])]++
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ANSI colors for the CLI diff output; enabled only when stdout is a
// terminal, so piped output stays plain.
const (
	ansiGreen  = "\x1b[32m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiReset  = "\x1b[0m"
)

// FormatDiff renders a DiffResult as readable claim-level output: claims
// added, removed, or changed between the two runs.
func FormatDiff(res *DiffResult, color bool) string {
	green, red, yellow := "+", "-", "~"
	if color {
		green, red, yellow = ansiGreen+"+", ansiRed+"-", ansiYellow+"~"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s -> %s\n", res.SessionA, res.SessionB)
	if !res.UpdatedAtA.IsZero() || !res.UpdatedAtB.IsZero() {
		fmt.Fprintf(&b, "  %s -> %s\n", formatTime(res.UpdatedAtA), formatTime(res.UpdatedAtB))
	}
	if len(res.Added)+len(res.Removed)+len(res.Changed) == 0 {
		b.WriteString("  no claim-level changes between these runs\n")
		return b.String()
	}

	for _, ch := range res.Changed {
		fmt.Fprintf(&b, "%s changed  %s  %s\n", yellow, kindTag(ch.To.Kind, color), firstLine(ch.To.Text))
		fmt.Fprintf(&b, "  was:      %s\n", firstLine(ch.From.Text))
	}
	for _, c := range res.Added {
		fmt.Fprintf(&b, "%s added    %s  %s\n", green, kindTag(c.Kind, color), firstLine(c.Text))
	}
	for _, c := range res.Removed {
		fmt.Fprintf(&b, "%s removed  %s  %s\n", red, kindTag(c.Kind, color), firstLine(c.Text))
	}
	fmt.Fprintf(&b, "  %d claims unchanged\n", res.Unchanged)
	return b.String()
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "(never pinned)"
	}
	return t.UTC().Format("2006-01-02 15:04:05Z07:00")
}

func kindTag(k Kind, color bool) string {
	tag := fmt.Sprintf("%-8s", "["+string(k)+"]")
	if !color {
		return tag
	}
	return tag + ansiReset
}

// firstLine reduces a claim to its first line for one-line diff rows.
func firstLine(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i] + " …"
	}
	return text
}
