package game

import (
	"time"
)

// Terminal is the in-game console overlay. When Open, the game grabs text
// input and renders a translucent panel at the bottom of the screen.
//
// The struct lives on the Game and is mutated under game.mu — all public
// methods assume the caller already holds that mutex.
type Terminal struct {
	open       bool
	input      []rune
	cursor     int      // rune index within `input`
	history    []string // executed commands, oldest first
	historyIdx int      // navigation index into history (-1 = editing new line)
	draft      []rune   // saved input when browsing history so Down can restore it
	output     []terminalLine

	// swallowNextChar is set by the key callback when a key press opens the
	// terminal (T or /). GLFW dispatches the matching char event right after,
	// which would otherwise insert the trigger character into the input.
	swallowNextChar bool

	// Rendering helpers lazily allocated in terminal_render.go.
	renderState *terminalRenderState
}

type terminalLine struct {
	text  string
	color RGBA
	at    time.Time
}

const terminalMaxOutput = 64
const terminalMaxInputRunes = 256

// Open shows the terminal and resets the history cursor to "editing new
// line" so Up/Down navigation starts at the latest command.
func (t *Terminal) Open() {
	t.open = true
	t.historyIdx = -1
	t.draft = t.draft[:0]
}

// Close hides the terminal and wipes the in-progress line. Executed commands
// are preserved in history so Up still works next time the terminal opens.
func (t *Terminal) Close() {
	t.open = false
	t.input = t.input[:0]
	t.cursor = 0
	t.historyIdx = -1
	t.draft = t.draft[:0]
	t.swallowNextChar = false
}

// IsOpen reports whether the terminal is currently capturing input.
func (t *Terminal) IsOpen() bool { return t.open }

// Input returns the current edited rune slice (read-only).
func (t *Terminal) Input() []rune { return t.input }

// CursorIndex returns the caret position as a rune index.
func (t *Terminal) CursorIndex() int { return t.cursor }

// InsertRune inserts r at the cursor and advances the caret.
func (t *Terminal) InsertRune(r rune) {
	if len(t.input) >= terminalMaxInputRunes {
		return
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
	if t.cursor > len(t.input) {
		t.cursor = len(t.input)
	}
	t.input = append(t.input, 0)
	copy(t.input[t.cursor+1:], t.input[t.cursor:])
	t.input[t.cursor] = r
	t.cursor++
}

// Backspace deletes the rune to the left of the caret.
func (t *Terminal) Backspace() {
	if t.cursor <= 0 || len(t.input) == 0 {
		return
	}
	t.input = append(t.input[:t.cursor-1], t.input[t.cursor:]...)
	t.cursor--
}

// Delete removes the rune to the right of the caret.
func (t *Terminal) Delete() {
	if t.cursor >= len(t.input) {
		return
	}
	t.input = append(t.input[:t.cursor], t.input[t.cursor+1:]...)
}

// CursorLeft moves the caret one rune to the left (clamped).
func (t *Terminal) CursorLeft() {
	if t.cursor > 0 {
		t.cursor--
	}
}

// CursorRight moves the caret one rune to the right (clamped).
func (t *Terminal) CursorRight() {
	if t.cursor < len(t.input) {
		t.cursor++
	}
}

// CursorHome moves the caret to the start of the input line.
func (t *Terminal) CursorHome() { t.cursor = 0 }

// CursorEnd moves the caret past the last rune.
func (t *Terminal) CursorEnd() { t.cursor = len(t.input) }

// HistoryUp walks one step into older history entries. The first call also
// stashes the in-progress draft so HistoryDown can restore it.
func (t *Terminal) HistoryUp() {
	if len(t.history) == 0 {
		return
	}
	if t.historyIdx == -1 {
		t.draft = append(t.draft[:0], t.input...)
		t.historyIdx = len(t.history) - 1
	} else if t.historyIdx > 0 {
		t.historyIdx--
	} else {
		return
	}
	t.setInput([]rune(t.history[t.historyIdx]))
}

// HistoryDown walks toward the newest entry and eventually restores the draft.
func (t *Terminal) HistoryDown() {
	if t.historyIdx == -1 {
		return
	}
	if t.historyIdx < len(t.history)-1 {
		t.historyIdx++
		t.setInput([]rune(t.history[t.historyIdx]))
		return
	}
	// Off the end — restore the draft and leave history mode.
	t.historyIdx = -1
	t.setInput(append([]rune(nil), t.draft...))
}

func (t *Terminal) setInput(r []rune) {
	t.input = append(t.input[:0], r...)
	t.cursor = len(t.input)
}

// Commit returns the trimmed input and clears it. The raw command is pushed
// onto history (if non-empty and different from the previous entry) so
// HistoryUp brings it back.
func (t *Terminal) Commit() string {
	line := string(t.input)
	t.input = t.input[:0]
	t.cursor = 0
	t.historyIdx = -1
	t.draft = t.draft[:0]

	trimmed := trimSpace(line)
	if trimmed == "" {
		return ""
	}
	if n := len(t.history); n == 0 || t.history[n-1] != trimmed {
		t.history = append(t.history, trimmed)
	}
	return trimmed
}

// AddOutput appends a single line to the scrollback and trims the backlog
// so it can't grow without bound.
func (t *Terminal) AddOutput(text string, color RGBA) {
	t.output = append(t.output, terminalLine{text: text, color: color, at: time.Now()})
	if len(t.output) > terminalMaxOutput {
		t.output = t.output[len(t.output)-terminalMaxOutput:]
	}
}

// trimSpace mirrors strings.TrimSpace but avoids the dependency on this tiny
// file (keeps the terminal struct package-local and testable).
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
