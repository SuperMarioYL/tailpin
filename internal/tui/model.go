// Package tui implements the tailpin dual-pane terminal UI: the pinned
// answer pane always shows the agent's current best conclusion claim by
// claim, and the span viewer shows the exact raw transcript records behind
// any claim. The TUI is strictly read-only against the transcript; the only
// writes are tailpin's own pin artifacts, through the tracker.
package tui

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"github.com/SuperMarioYL/tailpin/internal/pin"
	"github.com/SuperMarioYL/tailpin/internal/transcript"
)

// sessionLoadedMsg carries a freshly parsed session: the initial load, and
// every later append the watcher picks up.
type sessionLoadedMsg struct {
	session *transcript.Session
	err     error
}

// Model is the tailpin TUI state.
type Model struct {
	path    string
	watchCh <-chan *transcript.Session
	store   *pin.Store

	session *transcript.Session
	answer  *pin.PinnedAnswer

	// tracker persists answer revisions; it is created on first load so it is
	// keyed by the session ID the transcript itself carries.
	tracker *pin.Tracker

	selected     int // claim under the cursor
	viewerOpen   bool
	viewerScroll int

	width, height int
	status        string // transient error line, shown in the header
}

// New builds the model for one session file. watchCh is the append watcher
// (nil for a static, already-complete transcript); store persists answer
// revisions and may be nil to run purely read-only (nothing is written).
func New(path string, watchCh <-chan *transcript.Session, store *pin.Store) Model {
	return Model{path: path, watchCh: watchCh, store: store}
}

// Init renders the transcript already on disk immediately, then keeps
// listening for appends.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.load(), waitForSession(m.watchCh))
}

func (m Model) load() tea.Cmd {
	return func() tea.Msg {
		sess, err := transcript.Load(m.path)
		return sessionLoadedMsg{session: sess, err: err}
	}
}

// waitForSession blocks until the watcher reports an appended batch, and
// returns Quit when the watch channel closes.
func waitForSession(ch <-chan *transcript.Session) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		sess, ok := <-ch
		if !ok {
			return tea.Quit
		}
		return sessionLoadedMsg{session: sess}
	}
}

// Update handles window sizing, session refreshes and the key bindings:
// j/k move between claims (or scroll the span viewer once open), Enter opens
// the raw span of the selected claim, Esc closes it, q quits.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case sessionLoadedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, waitForSession(m.watchCh)
		}
		m.session = msg.session
		m.status = ""
		if m.tracker == nil && m.store != nil {
			m.tracker = pin.NewTracker(m.store, msg.session.SessionID)
		}
		pa := pin.Extract(msg.session.SessionID, msg.session.Spans)
		if pa != nil && m.tracker != nil {
			if err := m.tracker.Observe(pa); err != nil {
				m.status = err.Error()
			}
		}
		m.answer = pa
		if m.selected >= len(m.claims()) {
			m.selected = len(m.claims()) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
		return m, waitForSession(m.watchCh)

	case tea.KeyMsg:
		switch key := msg.String(); key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.viewerOpen = false
		case "up", "k":
			if m.viewerOpen {
				if m.viewerScroll > 0 {
					m.viewerScroll--
				}
			} else if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.viewerOpen {
				m.viewerScroll++
			} else if m.selected < len(m.claims())-1 {
				m.selected++
			}
		case "enter":
			if !m.viewerOpen && len(m.claims()) > 0 {
				m.viewerOpen = true
				m.viewerScroll = 0
			}
		}
	}
	return m, nil
}

func (m Model) claims() []pin.Claim {
	if m.answer == nil {
		return nil
	}
	return m.answer.Claims
}

// Terminal size assumed when none is known (piped output never receives a
// WindowSizeMsg); the first real resize overrides it.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

// View draws the frame: header, then the pinned-answer pane (and the span
// viewer below it once a claim is opened), then the key hints. The output is
// truncated to the terminal size so the panes never scroll the screen.
func (m Model) View() string {
	width, height := m.width, m.height
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	headerLines := 2
	footerLines := 1
	bodyHeight := height - headerLines - footerLines
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	var body string
	if m.viewerOpen {
		spanHeight := bodyHeight * 3 / 5
		if spanHeight < 3 {
			spanHeight = 3
		}
		answerHeight := bodyHeight - spanHeight
		if answerHeight < 1 {
			answerHeight = 1
		}
		body = m.answerPane(width, answerHeight) + "\n" +
			m.viewerPane(width, spanHeight)
	} else {
		body = m.answerPane(width, bodyHeight)
	}

	return m.headerView(width) + "\n" + body + "\n" + m.footerView()
}

func (m Model) headerView(width int) string {
	line1 := titleStyle.Render("tailpin")
	if m.session != nil {
		line1 += " · " + shortUUID(m.session.SessionID)
	}
	if m.answer != nil {
		rev := 0
		if m.tracker != nil {
			rev = m.tracker.Revision()
		}
		line1 += dimStyle.Render(" · rev " + strconv.Itoa(rev) + " · " + string(m.answer.Confidence))
		if !m.answer.UpdatedAt.IsZero() {
			line1 += dimStyle.Render(" · " + m.answer.UpdatedAt.UTC().Format("15:04:05"))
		}
	}
	line2 := dimStyle.Render(m.path)
	if m.status != "" {
		line2 += "  " + errorStyle.Render(m.status)
	}
	return line1 + "\n" + truncateLines(line2, width)
}

func (m Model) footerView() string {
	if m.viewerOpen {
		return dimStyle.Render("j/k scroll · esc back · q quit")
	}
	return dimStyle.Render("j/k claims · enter span · q quit")
}

// answerPane renders the pinned answer as a list of claims, each with its
// provenance label, windowed so the selected claim stays visible.
func (m Model) answerPane(width, height int) string {
	claims := m.claims()
	if len(claims) == 0 {
		return dimStyle.Render("waiting for the agent's first conclusion…")
	}
	blocks := make([][]string, len(claims))
	for i, c := range claims {
		blocks[i] = renderClaim(c, i == m.selected, width)
	}
	var lines []string
	for _, block := range windowBlocks(blocks, m.selected, height) {
		lines = append(lines, block...)
	}
	return truncateLines(strings.Join(lines, "\n"), height)
}

// viewerPane renders the span viewer for the selected claim with a fixed
// header line and a scrollable body.
func (m Model) viewerPane(width, height int) string {
	claims := m.claims()
	if len(claims) == 0 {
		return paneHeader("span", width)
	}
	content := viewerContent(m.session, claims[m.selected], width)
	maxScroll := len(content) - (height - 1)
	scroll := m.viewerScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	body := content
	if maxScroll > 0 {
		body = content[scroll:]
	}
	return paneHeader("span", width) + "\n" + truncateLines(strings.Join(body, "\n"), height-1)
}

// renderClaim renders one claim block: selection marker, kind tag, wrapped
// text with a hanging indent, and the provenance label.
func renderClaim(c pin.Claim, selected bool, width int) []string {
	marker := "  "
	textStyle := lipgloss.NewStyle()
	if selected {
		marker = "❯ "
		textStyle = selectedStyle
	}
	tag := kindTag(c.Kind)
	prefixWidth := lipgloss.Width(marker) + lipgloss.Width(tag) + 1
	indent := strings.Repeat(" ", prefixWidth)
	wrapWidth := width - prefixWidth
	if wrapWidth < 10 {
		wrapWidth = 10
	}
	wrapped := lipgloss.NewStyle().Width(wrapWidth).Render(c.Text)

	var lines []string
	for i, line := range strings.Split(wrapped, "\n") {
		if i == 0 {
			lines = append(lines, marker+tag+" "+textStyle.Render(line))
		} else {
			lines = append(lines, indent+line)
		}
	}
	lines = append(lines, indent+dimStyle.Render(provenanceLabel(c.Span)))
	return lines
}

// windowBlocks picks a contiguous run of rendered claim blocks that fits
// height lines and includes the selected claim, so j/k always keeps the
// cursor on screen even with more claims than rows.
func windowBlocks(blocks [][]string, selected, height int) [][]string {
	if height <= 0 || len(blocks) == 0 {
		return nil
	}
	total := 0
	for _, b := range blocks {
		total += len(b)
	}
	if total <= height {
		return blocks
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= len(blocks) {
		selected = len(blocks) - 1
	}
	start, end := selected, selected
	used := len(blocks[selected])
	for start > 0 && used+len(blocks[start-1]) <= height {
		start--
		used += len(blocks[start])
	}
	for end+1 < len(blocks) && used+len(blocks[end+1]) <= height {
		end++
		used += len(blocks[end])
	}
	return blocks[start : end+1]
}

// truncateLines cuts a multi-line string to at most height lines.
func truncateLines(s string, height int) string {
	if height < 1 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= height {
		return s
	}
	return strings.Join(lines[:height], "\n")
}

// RunProgram wires the model into a bubbletea program over one session file
// with the default append-watch polling. When stdin is not a terminal, keys
// are read from stdin instead of /dev/tty, so the TUI can also be driven
// non-interactively (e.g. printf q | tailpin watch session.jsonl).
func RunProgram(path string, store *pin.Store) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watchCh := transcript.Watch(ctx, path, 0)
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if !term.IsTerminal(os.Stdin.Fd()) {
		opts = append(opts, tea.WithInput(os.Stdin))
	}
	p := tea.NewProgram(New(path, watchCh, store), opts...)
	_, err := p.Run()
	return err
}
