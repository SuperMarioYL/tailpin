// Command tailpin pins a coding agent's live conclusion over its
// Claude-Code-style JSONL session transcript, claim by claim, with every
// claim traceable to the raw record that produced it.
//
// Usage:
//
//	tailpin                       watch the most recently active session (TUI)
//	tailpin watch <file>          watch a session file (TUI)
//	tailpin watch <file> --plain  stream the pinned answer without the TUI
//	tailpin diff [idA idB]        claim-level diff between two runs
//
// tailpin is strictly read-only against the transcript; the only files it
// writes are its own pin artifacts under ~/.tailpin/sessions.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/SuperMarioYL/tailpin/internal/pin"
	"github.com/SuperMarioYL/tailpin/internal/transcript"
	"github.com/SuperMarioYL/tailpin/internal/tui"
)

// version is the single release identity of the binary.
const version = "0.1.0"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "tailpin:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return watchLatest(stdout)
	}
	switch args[0] {
	case "watch":
		return runWatch(ctx, args[1:], stdout)
	case "diff":
		return runDiffCmd(args[1:], stdout, isTTY(stdout))
	case "help", "-h", "--help":
		usage(stdout)
		return nil
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "tailpin %s\n", version)
		return nil
	default:
		return fmt.Errorf("unknown command %q — try 'tailpin help'", args[0])
	}
}

// runWatch implements `tailpin watch <file> [--plain]`; the flag may come
// before or after the file.
func runWatch(ctx context.Context, args []string, stdout io.Writer) error {
	var (
		plain bool
		file  string
	)
	for _, arg := range args {
		switch arg {
		case "--plain", "-plain":
			plain = true
		default:
			if file != "" {
				return errors.New("usage: tailpin watch <file.jsonl> [--plain]")
			}
			file = arg
		}
	}
	if file == "" {
		return errors.New("watch requires a session file: tailpin watch <file.jsonl> [--plain]")
	}
	if _, err := os.Stat(file); err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	if plain {
		storeDir, err := pin.DefaultDir()
		if err != nil {
			return err
		}
		return watchPlain(ctx, file, storeDir, stdout, isTTY(stdout))
	}
	store, err := pin.NewStore(mustStoreDir())
	if err != nil {
		return err
	}
	return tui.RunProgram(file, store)
}

// watchLatest implements bare `tailpin`: auto-discover the most recently
// active session and watch it in the TUI, with no flags.
func watchLatest(stdout io.Writer) error {
	root, err := transcript.DefaultRoot()
	if err != nil {
		return err
	}
	ref, err := transcript.Latest(root)
	if err != nil {
		return fmt.Errorf("%v — pass a session explicitly: tailpin watch <file.jsonl>", err)
	}
	fmt.Fprintf(stdout, "watching %s\n", ref.Path)
	store, err := pin.NewStore(mustStoreDir())
	if err != nil {
		return err
	}
	return tui.RunProgram(ref.Path, store)
}

// watchPlain streams the pinned answer to a plain terminal: the answer
// redraws in place on a TTY (or appends with a header when piped), and every
// claim line prints its line-range provenance.
func watchPlain(ctx context.Context, path, storeDir string, out io.Writer, redraw bool) error {
	sess, err := transcript.Load(path)
	if err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	store, err := pin.NewStore(storeDir)
	if err != nil {
		return err
	}
	tracker := pin.NewTracker(store, sess.SessionID)
	renderPlain(out, sess, tracker, redraw)

	updates := transcript.Watch(ctx, path, 0)
	for {
		select {
		case <-ctx.Done():
			return nil
		case sess, ok := <-updates:
			if !ok {
				return nil
			}
			renderPlain(out, sess, tracker, redraw)
		}
	}
}

func renderPlain(out io.Writer, sess *transcript.Session, tracker *pin.Tracker, redraw bool) {
	if redraw {
		fmt.Fprint(out, "\x1b[H\x1b[2J") // cursor home + clear, for in-place redraw
	}
	pa := pin.Extract(sess.SessionID, sess.Spans)
	if pa != nil {
		if err := tracker.Observe(pa); err != nil {
			fmt.Fprintf(out, "tailpin: %v\n", err)
		}
	}
	if pa == nil {
		fmt.Fprintf(out, "tailpin · %s\nwaiting for the agent's first conclusion…\n\n", sess.SessionID)
		return
	}
	updated := ""
	if !pa.UpdatedAt.IsZero() {
		updated = " · " + pa.UpdatedAt.UTC().Format("15:04:05")
	}
	fmt.Fprintf(out, "tailpin · %s · rev %d · %s%s\n", sess.SessionID, tracker.Revision(), pa.Confidence, updated)
	fmt.Fprintf(out, "%s\n\n", sess.Path)
	for _, c := range pa.Claims {
		lines := strings.Split(c.Text, "\n")
		fmt.Fprintf(out, "  %-7s %s\n", lineRange(c.Span), lines[0])
		for _, l := range lines[1:] {
			fmt.Fprintf(out, "          %s\n", l)
		}
	}
}

func lineRange(s pin.Span) string {
	if s.LineEnd != s.LineStart {
		return fmt.Sprintf("L%d-%d", s.LineStart, s.LineEnd)
	}
	return fmt.Sprintf("L%d", s.LineStart)
}

// runDiffCmd implements `tailpin diff [idA idB]`: with no arguments it diffs
// the newest session against the previous session in the same project; with
// two IDs it diffs those two sessions explicitly.
func runDiffCmd(args []string, out io.Writer, color bool) error {
	root, err := transcript.DefaultRoot()
	if err != nil {
		return err
	}
	return diffSessions(root, mustStoreDir(), args, out, color)
}

// diffSessions resolves the two sessions to compare (under root) and prints
// their claim-level diff.
func diffSessions(root, storeDir string, args []string, out io.Writer, color bool) error {
	if len(args) > 2 {
		return errors.New("usage: tailpin diff [idA idB]")
	}
	refs, err := transcript.Sessions(root)
	if err != nil {
		return fmt.Errorf("discover sessions under %s: %w", root, err)
	}
	if len(refs) == 0 {
		return fmt.Errorf("%w under %s", transcript.ErrNoSessions, root)
	}

	var refA, refB transcript.Ref
	switch len(args) {
	case 0:
		refB = refs[0] // newest session overall
		refA, err = previousInProject(refs, refB)
		if err != nil {
			return err
		}
	case 2:
		refA, err = resolveRef(refs, args[0])
		if err != nil {
			return err
		}
		refB, err = resolveRef(refs, args[1])
		if err != nil {
			return err
		}
	case 1:
		return errors.New("diff needs either no session IDs or two: tailpin diff [idA idB]")
	}

	store, err := pin.NewStore(storeDir)
	if err != nil {
		return err
	}
	res := pin.Diff(answerFor(store, refA), answerFor(store, refB))
	fmt.Fprint(out, pin.FormatDiff(res, color))
	return nil
}

// answerFor returns a session's pinned answer: the stored artifact when the
// session was watched, otherwise the answer extracted from its transcript —
// so diff works on any two past sessions, watched or not.
func answerFor(store *pin.Store, ref transcript.Ref) *pin.PinnedAnswer {
	if pa, err := store.Load(ref.SessionID); err == nil {
		return pa
	}
	sess, err := transcript.Load(ref.Path)
	if err != nil {
		return nil
	}
	return pin.Extract(ref.SessionID, sess.Spans)
}

func resolveRef(refs []transcript.Ref, id string) (transcript.Ref, error) {
	for _, r := range refs {
		if r.SessionID == id {
			return r, nil
		}
	}
	return transcript.Ref{}, fmt.Errorf("session %q not found — ids are the .jsonl file names under the projects root", id)
}

// previousInProject returns the next-older session in the same project as
// newest; refs arrive sorted newest first.
func previousInProject(refs []transcript.Ref, newest transcript.Ref) (transcript.Ref, error) {
	for _, r := range refs {
		if r.Project == newest.Project && r.Path != newest.Path {
			return r, nil
		}
	}
	return transcript.Ref{}, fmt.Errorf("no previous session found in project %s", newest.Project)
}

func mustStoreDir() string {
	dir, err := pin.DefaultDir()
	if err != nil {
		return "" // NewStore will surface the failure
	}
	return dir
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func usage(out io.Writer) {
	fmt.Fprintf(out, `tailpin %s — pin your coding agent's live conclusion over its session transcript

Usage:
  tailpin                         watch the most recently active session (TUI)
  tailpin watch <file>            watch a session file (TUI)
  tailpin watch <file> --plain    stream the pinned answer without the TUI
  tailpin diff [idA idB]          claim-level diff between two runs
  tailpin help | version          this help | the version

Keys: j/k move between claims · enter opens the raw transcript span behind a
claim · esc goes back · q quits. Every claim carries the line range of the
record it came from; tailpin never writes the transcript, only its own pin
artifacts under ~/.tailpin/sessions.
`, version)
}
