package game

import "testing"

func TestTerminalInsertAndBackspace(t *testing.T) {
	term := &Terminal{}
	term.Open()
	for _, r := range "/tp 1 2" {
		term.InsertRune(r)
	}
	if got, want := string(term.Input()), "/tp 1 2"; got != want {
		t.Fatalf("input mismatch: got %q want %q", got, want)
	}
	if term.CursorIndex() != 7 {
		t.Fatalf("cursor at %d, want 7", term.CursorIndex())
	}
	term.Backspace()
	term.Backspace()
	if got, want := string(term.Input()), "/tp 1"; got != want {
		t.Fatalf("after backspace: got %q want %q", got, want)
	}
}

func TestTerminalCursorMotion(t *testing.T) {
	term := &Terminal{}
	term.Open()
	for _, r := range "abc" {
		term.InsertRune(r)
	}
	term.CursorHome()
	term.InsertRune('>')
	if got, want := string(term.Input()), ">abc"; got != want {
		t.Fatalf("home-insert: got %q want %q", got, want)
	}
	term.CursorEnd()
	term.InsertRune('!')
	if got, want := string(term.Input()), ">abc!"; got != want {
		t.Fatalf("end-insert: got %q want %q", got, want)
	}
	term.CursorLeft()
	term.CursorLeft()
	term.Delete()
	if got, want := string(term.Input()), ">ab!"; got != want {
		t.Fatalf("delete: got %q want %q", got, want)
	}
}

func TestTerminalHistoryRoundtrip(t *testing.T) {
	term := &Terminal{}
	term.Open()
	for _, r := range "first" {
		term.InsertRune(r)
	}
	if got := term.Commit(); got != "first" {
		t.Fatalf("commit 1: got %q", got)
	}
	term.Open()
	for _, r := range "second" {
		term.InsertRune(r)
	}
	if got := term.Commit(); got != "second" {
		t.Fatalf("commit 2: got %q", got)
	}
	term.Open()
	for _, r := range "draft" {
		term.InsertRune(r)
	}
	term.HistoryUp()
	if got := string(term.Input()); got != "second" {
		t.Fatalf("history up 1: got %q", got)
	}
	term.HistoryUp()
	if got := string(term.Input()); got != "first" {
		t.Fatalf("history up 2: got %q", got)
	}
	term.HistoryDown()
	if got := string(term.Input()); got != "second" {
		t.Fatalf("history down 1: got %q", got)
	}
	term.HistoryDown()
	if got := string(term.Input()); got != "draft" {
		t.Fatalf("history down restore: got %q", got)
	}
}

func TestDispatchCommandUnknown(t *testing.T) {
	g := &Game{player: newPlayer(), terminal: &Terminal{}}
	g.mu.Lock()
	defer g.mu.Unlock()
	out, ok := dispatchCommand(g, "/bogus")
	if !ok {
		t.Fatalf("expected handled=true for unknown command")
	}
	if out == "" {
		t.Fatalf("expected error message for unknown command")
	}
}

func TestDispatchCommandEmpty(t *testing.T) {
	g := &Game{player: newPlayer(), terminal: &Terminal{}}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := dispatchCommand(g, "   "); ok {
		t.Fatalf("expected handled=false for empty input")
	}
}

func TestDispatchCommandHelp(t *testing.T) {
	g := &Game{player: newPlayer(), terminal: &Terminal{}}
	g.mu.Lock()
	defer g.mu.Unlock()
	out, ok := dispatchCommand(g, "/help")
	if !ok || out == "" {
		t.Fatalf("/help returned no output")
	}
	outTp, ok := dispatchCommand(g, "/help tp")
	if !ok || outTp == "" {
		t.Fatalf("/help tp returned no output")
	}
}

func TestDispatchCommandPos(t *testing.T) {
	g := &Game{player: newPlayer(), terminal: &Terminal{}}
	g.player.pos = FPoint3D{1, 2, 3}
	g.mu.Lock()
	defer g.mu.Unlock()
	out, ok := dispatchCommand(g, "/pos")
	if !ok || out == "" {
		t.Fatalf("/pos returned nothing")
	}
}

func TestDispatchCommandGamemode(t *testing.T) {
	g := &Game{player: newPlayer(), terminal: &Terminal{}}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := dispatchCommand(g, "/gm creative"); !ok {
		t.Fatalf("/gm creative not handled")
	}
	if _, ok := dispatchCommand(g, "/gm survival"); !ok {
		t.Fatalf("/gm survival not handled")
	}
	if out, _ := dispatchCommand(g, "/gm nonsense"); out == "" {
		t.Fatalf("expected error for bad gamemode arg")
	}
}
