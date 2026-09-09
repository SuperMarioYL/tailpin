// Package transcript parses Claude-Code-style JSONL session transcripts and
// lifts the assistant text spans a pinned answer can be extracted from.
//
// The dialect owned here is the one Claude Code (and Claude-Code-style
// harnesses such as Opencode) writes under ~/.claude/projects: one JSON
// record per line, where message records carry type "user" or "assistant",
// a uuid, and an ISO 8601 timestamp with a Z suffix. The harness interleaves
// non-message records into the same file (mode, queue-operation,
// file-history-snapshot, attachment, last-prompt, custom-title, system, ...)
// and nests subagent traces and journals in subdirectories; the adapter
// skips both instead of assuming every line is a message.
//
// Parsing is read-only and tolerant: a trailing partial line (a record the
// harness is still appending) or an unparseable line is skipped, never fatal.
package transcript

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// pollInterval is the append-only poll cadence from the plan: 250ms, so a
// mid-run answer refreshes within ~1s of each appended batch.
const pollInterval = 250 * time.Millisecond

// TextSpan is one assistant text block lifted out of a session file, with the
// provenance needed to jump back to the exact raw transcript record that
// produced it. Line numbers are 1-based and inclusive; byte offsets are
// 0-based file offsets of the record's first byte and just past its last
// content byte (excluding the line terminator).
type TextSpan struct {
	File        string
	LineStart   int
	LineEnd     int
	ByteStart   int
	ByteEnd     int
	MessageUUID string
	Timestamp   string // ISO 8601 with Z suffix, as written by the harness
	Text        string
}

// Session is the parsed view of one session file: the lifted assistant text
// spans in file order, plus the raw lines so a viewer can show the exact
// source span. Tailpin never writes any of this back.
type Session struct {
	Path      string
	SessionID string
	Spans     []TextSpan
	Lines     []string
}

// messageRecord is the subset of a JSONL record the dialect defines for
// message entries; every other record type is skipped by type, not by shape.
type messageRecord struct {
	Type        string          `json:"type"`
	UUID        string          `json:"uuid"`
	SessionID   string          `json:"sessionId"`
	Timestamp   string          `json:"timestamp"`
	IsSidechain bool            `json:"isSidechain"`
	Message     json.RawMessage `json:"message"`
}

type messageBody struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// isMessageRecord reports whether a record type carries a conversation
// message. The allow-list is deliberate: harness bookkeeping records are
// skipped by never matching, so new non-message types cost nothing.
func isMessageRecord(recordType string) bool {
	return recordType == "user" || recordType == "assistant"
}

// Load reads and parses a session file. An unreadable or missing file is an
// error; a readable file with no assistant text yields a Session with no
// spans. The SessionID is taken from the first record that carries one and
// falls back to the file base name.
func Load(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sess := &Session{Path: path, SessionID: sessionIDFromPath(path)}
	sess.Lines = splitLines(string(data))
	fallbackID := sess.SessionID
	offset := 0
	for i, line := range sess.Lines {
		byteStart, byteEnd := offset, offset+len(line)
		offset = byteEnd + 1 // skip the line terminator
		trimmed := strings.TrimSuffix(line, "\r")
		if trimmed == "" {
			continue
		}
		var rec messageRecord
		if err := json.Unmarshal([]byte(trimmed), &rec); err != nil {
			continue // partial or unparseable line: the harness may still be appending
		}
		if !isMessageRecord(rec.Type) || rec.IsSidechain {
			continue
		}
		if rec.SessionID != "" && sess.SessionID == fallbackID {
			sess.SessionID = rec.SessionID
		}
		if rec.Type != "assistant" {
			continue
		}
		sess.Spans = append(sess.Spans, liftAssistantSpans(path, rec, i+1, byteStart, byteEnd)...)
	}
	return sess, nil
}

// liftAssistantSpans extracts the text blocks of one assistant record as
// spans pointing at the record's raw line and byte range.
func liftAssistantSpans(path string, rec messageRecord, lineNum, byteStart, byteEnd int) []TextSpan {
	var body messageBody
	if len(rec.Message) == 0 || json.Unmarshal(rec.Message, &body) != nil || body.Role != "assistant" {
		return nil
	}
	blocks, err := decodeContent(body.Content)
	if err != nil {
		return nil
	}
	var spans []TextSpan
	for _, b := range blocks {
		if b.Type != "text" || strings.TrimSpace(b.Text) == "" {
			continue
		}
		spans = append(spans, TextSpan{
			File:        path,
			LineStart:   lineNum,
			LineEnd:     lineNum,
			ByteStart:   byteStart,
			ByteEnd:     byteEnd,
			MessageUUID: rec.UUID,
			Timestamp:   rec.Timestamp,
			Text:        b.Text,
		})
	}
	return spans
}

// decodeContent decodes a message content field, which the dialect writes
// either as a plain string (user prompts) or as an array of typed blocks
// (assistant messages: thinking, text, tool_use, ...).
func decodeContent(raw json.RawMessage) ([]contentBlock, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if strings.TrimSpace(s) == "" {
			return nil, nil
		}
		return []contentBlock{{Type: "text", Text: s}}, nil
	}
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}
	return blocks, nil
}

func sessionIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// splitLines splits data into lines without their terminators. A trailing
// newline does not produce a final empty line.
func splitLines(data string) []string {
	lines := strings.Split(data, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// Watch polls path every interval and sends a freshly parsed Session on the
// returned channel whenever the file grows or changes. The change baseline is
// captured synchronously before Watch returns, so every append after the
// Watch call is detected no matter when the polling goroutine is first
// scheduled. The caller renders the initial state itself via Load; Watch only
// reports later appends, which keeps the channel free of a duplicate first
// frame. The channel closes when ctx is done. A read error during polling is
// retried on the next tick.
func Watch(ctx context.Context, path string, interval time.Duration) <-chan *Session {
	if interval <= 0 {
		interval = pollInterval
	}
	ch := make(chan *Session, 1)
	size, mod, ok := statFile(path)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			newSize, newMod, newOK := statFile(path)
			if !newOK || (ok && newSize == size && mod.Equal(newMod)) {
				continue
			}
			sess, err := Load(path)
			if err != nil {
				continue
			}
			size, mod, ok = newSize, newMod, true
			select {
			case ch <- sess:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func statFile(path string) (int64, time.Time, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, time.Time{}, false
	}
	return info.Size(), info.ModTime(), true
}
