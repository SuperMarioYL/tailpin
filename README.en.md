**English** | [简体中文](./README.md)

<div align="center">

<img src="assets/icons/pin.svg" width="42" alt="tailpin" />

# tailpin

**Pin your coding agent's latest conclusion to the top of your terminal**

<img src="https://readme-typing-svg.demolab.com?font=Fira%20Code&size=24&pause=1300&color=4493F8&center=true&vCenter=true&width=640&height=44&lines=pin%20your%20coding%20agent%27s%20live%20conclusion;every%20claim%20jumps%20back%20to%20its%20raw%20transcript%20record;tailpin%20diff%20compares%20two%20runs%20of%20the%20same%20task" alt="tailpin" />

[![Go Version](https://img.shields.io/badge/go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![CI](https://img.shields.io/github/actions/workflow/status/SuperMarioYL/tailpin/ci.yml?branch=main&label=CI)](https://github.com/SuperMarioYL/tailpin/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/SuperMarioYL/tailpin?include_prereleases)](https://github.com/SuperMarioYL/tailpin/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/SuperMarioYL/tailpin.svg)](https://pkg.go.dev/github.com/SuperMarioYL/tailpin)
[![Go Report Card](https://goreportcard.com/badge/github.com/SuperMarioYL/tailpin)](https://goreportcard.com/report/github.com/SuperMarioYL/tailpin)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](./LICENSE)

Free and open source (MIT) · strictly read-only · fully offline and deterministic · no API key · no config

</div>

---

## <img src="assets/icons/rocket.svg" width="20" /> Quickstart

Install (Go 1.24+):

```sh
go install github.com/SuperMarioYL/tailpin@latest
```

or grab a macOS / Linux binary from [Releases](https://github.com/SuperMarioYL/tailpin/releases) (every version ships with sha256 checksums).

Walk the whole flow with the two example sessions bundled in this repo — no Claude Code required:

```sh
git clone https://github.com/SuperMarioYL/tailpin.git
cd tailpin/examples

# 1) Plain mode: the answer pins immediately, each claim tagged with its L20 provenance
tailpin watch projects/-Users-dev-work-api-server/run-b.jsonl --plain

# 2) TUI: j/k move between claims · Enter jumps to the raw record behind one · esc back · q quit
tailpin watch projects/-Users-dev-work-api-server/run-b.jsonl

# 3) Cross-run diff: same task run twice, claims compared one by one
CLAUDE_CONFIG_DIR=. tailpin diff run-a run-b
```

Real `--plain` output (example session run-b, refreshing live while the session runs):

```text
tailpin · d8e9f0a1-2222-4bbb-8ccc-000000000002 · rev 1 · high
examples/projects/-Users-dev-work-api-server/run-b.jsonl

  L20     In summary, the flaky parser test is fixed for good:
  L20     Root cause: CountTokens in internal/parser/parser.go double-counted the trailing newline via the i == len(s) sentinel iteration.
  L20     Fix: corrected the loop boundary at parser.go:41 and added a regression case to parser_test.go.
  L20     Verified: go test ./... passes, 214/214.
  L20     Caveat: the config loader refactor is still pending.
```

On a real session it starts with zero arguments — tailpin auto-picks the most recently active session under `~/.claude/projects`; you can also watch any Claude-Code-style JSONL file directly (sessions written by Opencode and other harnesses that speak the same dialect work the same way):

```sh
tailpin                              # auto-discover the latest session (TUI)
tailpin watch path/to/session.jsonl  # any Claude-Code-style JSONL file
```

## <img src="assets/icons/video.svg" width="20" /> Demo

[assets/demo.cast](assets/demo.cast) is a real 26-second asciinema v2 recording: the pinned answer updating live as the agent's session grows, Enter jumping to the raw transcript span behind a claim, and tailpin diff comparing two runs' conclusions.

```sh
asciinema play assets/demo.cast
```

The TUI answer pane (❯ marks the selection; every claim carries `line · uuid · timestamp` provenance):

```text
tailpin · d8e9f0a1 · rev 1 · high · 20:34:21
examples/projects/-Users-dev-work-api-server/run-b.jsonl
❯ [fix]    In summary, the flaky parser test is fixed for good:
           L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
  [finding] Root cause: CountTokens in internal/parser/parser.go double-counted the trailing newline via the i == len(s) sentinel iteration.
            L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
  [fix]     Fix: corrected the loop boundary at parser.go:41 and added a regression case to parser_test.go.
            L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
  [finding] Verified: go test ./... passes, 214/214.
            L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
  [caveat]  Caveat: the config loader refactor is still pending.
            L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
j/k claims · enter span · q quit
```

Press Enter on a claim to open the raw transcript span that produced it (the producing record is highlighted; `…` marks sides cut to terminal width):

```text
── span ───────────────────────────────────────────────────────────────────────────────────────────────────────────
claim: Root cause: CountTokens in internal/parser/parser.go double-counted the trailing newline via the i == len(s) sentinel iteration.
L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
 12 {"type":"custom-title","customTitle":"fix flaky CountTokens test for good","sessionId":"d8e9f0a1-2222-4bbb-8ccc-000000000002"}
 13 {"type":"assistant","uuid":"d8e9f0a1-0000-4000-8000-000000000009","parentUuid":"d8e9f0a1-0000-4000-8000-000000000007","sessionId":"d8e…
 14 {"type":"assistant","uuid":"d8e9f0a1-0000-4000-8000-000000000010","parentUuid":"d8e9f0a1-0000-4000-8000-000000000009","sessionId":"d8e…
 15 {"type":"assistant","uuid":"d8e9f0a1-0000-4000-8000-000000000011","parentUuid":"d8e9f0a1-0000-4000-8000-000000000010","sessionId":"d8e…
 16 {"type":"user","uuid":"b8c1d2e3-0000-4000-8000-000000000012","parentUuid":"d8e9f0a1-0000-4000-8000-000000000011","sessionId":"d8e9f0a1…
 17 {"type":"system","parentUuid":"b8c1d2e3-0000-4000-8000-000000000012","isSidechain":false,"subtype":"stop_hook_summary","hookCount":1,"…
 18 {"type":"assistant","uuid":"d8e9f0a1-0000-4000-8000-000000000013","parentUuid":"b8c1d2e3-0000-4000-8000-000000000012","sessionId":"d8e…
 19 {"type":"last-prompt","lastPrompt":"run the full parser test suite when done","leafUuid":"d8e9f0a1-0000-4000-8000-000000000013","sessi…
 20 …rser test is fixed for good:\n\n- Root cause: CountTokens in internal/parser/parser.go double-counted the trailing newline via the i =…
j/k scroll · esc back · q quit
```

After re-running the same task, `tailpin diff` compares what the two runs concluded:

```text
run-a -> run-b
~ changed  [finding]  Verified: go test ./... passes, 214/214.
  was:      Verified: go test ./... passes locally, 214/214.
+ added    [fix]     In summary, the flaky parser test is fixed for good:
+ added    [finding]  Root cause: CountTokens in internal/parser/parser.go double-counted the trailing newline via the i == len(s) sentinel iteration.
+ added    [fix]     Fix: corrected the loop boundary at parser.go:41 and added a regression case to parser_test.go.
- removed  [finding]  Summary of the first investigation:
- removed  [finding]  Root cause: the test shares mutable parser state with TestCountTokensBasic and only flakes when the two run back to back.
- removed  [fix]     Fix: added a warm-cache guard that skips the test when the shared state is dirty.
  1 claims unchanged
```

## <img src="assets/icons/bulb.svg" width="20" /> Why tailpin

In a Claude Code session that runs to ten thousand lines, the coding agent's latest conclusion lives only at the transcript's tail. Wanting to know "what does it currently believe the answer is?" means scrolling a moving log interleaved with tool output — and on local long-reasoning models, the conclusion sits inside a long thinking stream on top of that. A forty-minute task may have ten lines you actually want to read.

tailpin turns those ten lines into a live, pinned answer panel:

- the conclusion is split into claims, each tagged finding / plan / fix / caveat;
- every claim jumps back to the exact raw transcript record that produced it (line, message uuid, timestamp);
- once you re-run the task, `tailpin diff` compares the two runs claim by claim: what is newly claimed, what was dropped, what was reworded.

It composes with prompt skills rather than competing with them: caveman or i-have-adhd-style skills change how the agent *talks*; tailpin changes *where you look*. It only reads the transcript already on disk — it never interferes with generation and never injects anything back into the agent's session.

## <img src="assets/icons/architecture.svg" width="20" /> How it works

One Go binary, four internal packages, zero model calls:

```text
~/.claude/projects/*/*.jsonl    (read-only; nested subagent / journal files never surface)
      |  append-only poll, 250ms
      v
transcript adapter    parse the JSONL dialect, lift assistant text spans
                     (non-message records skipped by type)
      v
pin extractor         deterministic ranking: recency + conclusion markers (EN/ZH)
                     + code blocks + file paths; permission asks, questions
                     and narration demoted
      v
pin store             atomic write to ~/.tailpin/sessions/<session_id>.pin.json
      v
TUI (bubbletea)       answer pane / span viewer           diff (CLI output)
```

The pinned answer is a separately addressable, revisioned artifact:

```text
PinnedAnswer { session_id, revision, updated_at, confidence, claims[] }
Claim        { text, kind: finding|plan|fix|caveat,
               span { file, line_start..line_end, byte_start..byte_end, message_uuid, timestamp } }
```

Three deliberate design decisions:

- **Strictly read-only.** tailpin never writes the harness's transcript; the only files it writes are its own pin artifacts, saved atomically (temp file + rename), so a kill mid-write can never leave a half-written JSON behind.
- **Deterministic and fully offline.** Conclusion extraction is heuristic scoring, not an LLM — no API key, no network. Heuristics can misread, which is exactly why every claim carries provenance: a wrong pick is visible in one keystroke, and the transcript stays ground truth.
- **Dialect isolation.** Only the adapter knows the Claude-Code-style JSONL dialect (message records, uuids, ISO 8601 timestamps; harness bookkeeping records like mode, queue-operation, file-history-snapshot are skipped by type). Supporting another transcript format means writing a new adapter, not touching the rest of the code.

## <img src="assets/icons/terminal-2.svg" width="20" /> Usage

| Command | What it does |
| --- | --- |
| `tailpin` | auto-discover the latest active session, open the TUI |
| `tailpin watch <file>` | watch a specific session file (TUI) |
| `tailpin watch <file> --plain` | plain-text live panel without the TUI (pipes and non-interactive terminals) |
| `tailpin diff` | newest session vs the previous session in the same project |
| `tailpin diff <idA> <idB>` | diff two sessions explicitly |
| `tailpin help` / `tailpin version` | help / version |

Keys: `j`/`k` move between claims (scrolls the span viewer once open) · `Enter` opens the raw record behind the selected claim · `esc` goes back · `q` quits.

Behavior details:

- Refreshes within about a second of each append (250ms polling); when the answer text changes, the revision bumps and the artifact lands in `~/.tailpin/sessions/<session_id>.pin.json`. Restarting tailpin continues the revision counter from disk.
- diff prefers stored pin artifacts; past sessions you never watched can still be diffed (extracted on the spot).
- The `CLAUDE_CONFIG_DIR` environment variable overrides the harness config directory (default `~/.claude`); the projects root becomes `<dir>/projects`.

v0.1 boundaries (deliberate):

- Read-only observability. Mid-run accept / annotate is a v0.2 candidate, gated on evidence that people actually keep tailpin open beside long runs.
- Only one transcript dialect: Claude-Code-style JSONL.
- One session per tailpin process; no web UI, no push notifications, no memory reinjection.
- Windows binaries are built with each release but untested on Windows.

## <img src="assets/icons/route.svg" width="20" /> Roadmap

- **v0.1 (current)**: pinned answer, per-claim provenance, dual-pane TUI, cross-run claim-level diff.
- **v0.2 (planned, gated on usage evidence)**: mid-run accept / annotate — accepting a conclusion becomes a metadata edit on the pin artifact, not a protocol change.
- **Explicitly not planned**: LLM-based extraction (offline determinism is a hard requirement), web UI / hosted dashboard, multi-session dashboards, reinjecting context into agent sessions, endless transcript-format support.

## <img src="assets/icons/license.svg" width="20" /> License

MIT · Copyright (c) 2026 SuperMarioYL
