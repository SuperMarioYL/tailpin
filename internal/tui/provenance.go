// Provenance rendering: everything needed to show, for one claim, the exact
// raw transcript records that produced it.
package tui

import (
	"fmt"
	"strings"

	"github.com/SuperMarioYL/tailpin/internal/pin"
	"github.com/SuperMarioYL/tailpin/internal/transcript"
)

// viewerContext is how many raw transcript lines are shown before and after
// the claim's span when the span viewer opens.
const viewerContext = 8

// anchorRunes caps the claim fragment used to locate the claim inside its raw
// record line. The first line of the claim is enough to find it, and a short
// anchor survives JSON escaping of the rest of the text.
const anchorRunes = 32

// spanWindow returns the 1-based inclusive line window to display around a
// span: the span itself plus context lines, clamped to the file.
func spanWindow(totalLines, lineStart, lineEnd, context int) (int, int) {
	if totalLines < 1 {
		return 0, 0
	}
	clampLine := func(n int) int {
		if n < 1 {
			return 1
		}
		if n > totalLines {
			return totalLines
		}
		return n
	}
	lineStart, lineEnd = clampLine(lineStart), clampLine(lineEnd)
	if lineStart > lineEnd {
		lineStart, lineEnd = lineEnd, lineStart
	}
	start, end := lineStart-context, lineEnd+context
	if start < 1 {
		start = 1
	}
	if end > totalLines {
		end = totalLines
	}
	return start, end
}

// provenanceLabel renders a claim's span as one compact line:
// "L20 · a1b0c2d3 · 2026-09-08T09:16:40.003Z". It is shown under every claim
// so the pinned answer always carries visible proof of where it came from.
func provenanceLabel(s pin.Span) string {
	lines := fmt.Sprintf("L%d", s.LineStart)
	if s.LineEnd != s.LineStart {
		lines = fmt.Sprintf("L%d-L%d", s.LineStart, s.LineEnd)
	}
	return fmt.Sprintf("%s · %s · %s", lines, shortUUID(s.MessageUUID), s.Timestamp)
}

func shortUUID(uuid string) string {
	if len(uuid) <= 8 {
		return uuid
	}
	return uuid[:8]
}

// claimAnchor returns the beginning of a claim's first line, used to locate
// the claim inside its raw JSONL record. The claim text is stored verbatim in
// the record; only newlines are escaped, and the first line has none.
func claimAnchor(claim string) string {
	first := claim
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	first = strings.TrimSpace(first)
	return truncateRunes(first, anchorRunes)
}

// frameLine horizontally frames one raw record line to fit the viewer width.
// Raw JSONL records are a single very long line, so the frame centers on the
// anchor (the claim being traced) and marks cut sides with an ellipsis; the
// record head (type, uuid, timestamp) shows when there is no anchor.
func frameLine(line, anchor string, width int) string {
	r := []rune(line)
	if len(r) <= width {
		return line
	}
	if width < 4 {
		return string(r[:width])
	}
	start := 0
	if anchor != "" {
		if i := strings.Index(line, anchor); i >= 0 {
			start = len([]rune(line[:i])) - width/4 // anchor a quarter into the frame
			if start < 0 {
				start = 0
			}
		}
	}
	if maxStart := len(r) - (width - 2); start > maxStart {
		start = maxStart
	}
	head, tail := "", ""
	if start > 0 {
		head = "…"
	}
	if start+width-2 < len(r) {
		tail = "…"
	}
	return head + string(r[start:start+width-2]) + tail
}

// highlightAnchor bolds the first occurrence of anchor inside an already
// framed line, so the claim is visibly pinpointed inside the raw record.
func highlightAnchor(framed, anchor string) string {
	if anchor == "" {
		return framed
	}
	i := strings.Index(framed, anchor)
	if i < 0 {
		return framed
	}
	return framed[:i] + anchorStyle.Render(anchor) + framed[i+len(anchor):]
}

// viewerContent renders the full span-viewer content for a claim: the claim,
// its provenance label, and the raw transcript records around the producing
// span with the producing records highlighted. Width is the pane width; the
// caller scrolls and truncates the returned lines.
func viewerContent(sess *transcript.Session, claim pin.Claim, width int) []string {
	var out []string
	out = append(out, "claim: "+selectedStyle.Render(claim.Text))
	out = append(out, dimStyle.Render(provenanceLabel(claim.Span)))
	if sess == nil || len(sess.Lines) == 0 {
		out = append(out, "", dimStyle.Render("transcript not loaded"))
		return out
	}

	start, end := spanWindow(len(sess.Lines), claim.Span.LineStart, claim.Span.LineEnd, viewerContext)
	gutterWidth := len(fmt.Sprintf("%d", len(sess.Lines))) + 1
	anchor := claimAnchor(claim.Text)
	frameWidth := width - gutterWidth - 1
	if frameWidth < 8 {
		frameWidth = 8
	}

	out = append(out, "")
	for n := start; n <= end; n++ {
		raw := sess.Lines[n-1]
		framed := frameLine(raw, anchor, frameWidth)
		gutter := dimStyle.Render(fmt.Sprintf("%*d ", gutterWidth, n))
		if n >= claim.Span.LineStart && n <= claim.Span.LineEnd {
			out = append(out, gutter+recordStyle.Render(highlightAnchor(framed, anchor)))
		} else {
			out = append(out, gutter+dimStyle.Render(framed))
		}
	}
	return out
}
