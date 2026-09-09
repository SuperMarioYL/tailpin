# Changelog

## 0.1.0 - 2026-09-09

First public release: pin a coding agent's live conclusion over its
Claude-Code-style JSONL session transcript, claim by claim, with every claim
traceable to the raw record that produced it.

### Live pinned answer

- Parse Claude-Code-style JSONL transcripts: assistant text spans lifted with
  byte-accurate provenance; harness bookkeeping records (mode,
  queue-operation, file-history-snapshot, ...) skipped by type; sidechain
  records and nested subagent/journal files excluded; a partial trailing line
  is tolerated, never fatal.
- `tailpin watch <file> --plain` streams the pinned answer to a plain
  terminal — in-place redraw on a TTY, header-separated appends when piped —
  with every claim printing its line-range provenance.
- Deterministic, offline extractor: recency, English and Chinese conclusion
  markers, code blocks and file-path mentions raise a span; permission asks,
  questions and narration are demoted. No model calls, no API key.
- 250 ms append-only polling; the answer refreshes within about a second of
  each appended batch.

### TUI with claim-to-span provenance

- Dual-pane bubbletea TUI: pinned-answer pane plus span viewer. `j`/`k` move
  between claims, Enter opens the exact raw transcript records behind a claim
  with the producing record highlighted, esc goes back, q quits.
- Session auto-discovery: bare `tailpin` picks the most recently active
  top-level session under `~/.claude/projects` (or `$CLAUDE_CONFIG_DIR`),
  with no flags.
- Atomic pin-artifact store under `~/.tailpin/sessions/<id>.pin.json`;
  revisions continue from disk across restarts. The transcript itself is
  never written.
- Piped stdin drives the TUI without a terminal size event (80x24 fallback
  rendering).

### Cross-run diff, demo and release

- `tailpin diff` prints claim-level added / removed / changed output
  (character-bigram pairing, works for Chinese claims); with no arguments it
  diffs the newest session against the previous session in the same project.
- Bilingual README (Chinese primary, English sibling) and a real asciinema v2
  demo cast over the bundled example sessions.
- Release pipeline: goreleaser builds linux/darwin/windows amd64/arm64
  tarballs plus sha256 checksums on every `v*.*.*` tag.
