package game

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	typhoon "github.com/TyphoonMC/TyphoonCore"
)

// commandHandler runs a single in-game command and returns one or more output
// lines (newline-separated). An empty return string means "no output".
type commandHandler struct {
	fn    func(game *Game, args []string) string
	usage string
	help  string
}

// commands is the public registry used by the in-game terminal. Populated
// in init() to break the initialisation cycle between the map and cmdHelp,
// which reads the map to list available commands. Handlers must be safe to
// call while game.mu is held by the caller — callers in input.go pass the
// mutex to dispatchCommand by convention.
var commands map[string]commandHandler

func init() {
	commands = map[string]commandHandler{
		"help": {fn: cmdHelp, usage: "/help [command]", help: "list commands or show detail for one"},
		"tp": {
			fn:    cmdTeleport,
			usage: "/tp <x> <y> <z>",
			help:  "teleport the player to the given world coordinates",
		},
		"sb": {
			fn:    cmdSetBlock,
			usage: "/sb <x> <y> <z> <id>",
			help:  "set the block at (x,y,z) to the given id (see /list)",
		},
		"setblock": {
			fn:    cmdSetBlock,
			usage: "/setblock <x> <y> <z> <id>",
			help:  "alias for /sb",
		},
		"gm": {
			fn:    cmdGamemode,
			usage: "/gm <survival|creative|spectator>",
			help:  "switch gamemode (shortcuts: s / c / sp)",
		},
		"gamemode": {
			fn:    cmdGamemode,
			usage: "/gamemode <survival|creative|spectator>",
			help:  "alias for /gm",
		},
		"stop": {fn: cmdStop, usage: "/stop", help: "quit the game cleanly"},
		"quit": {fn: cmdStop, usage: "/quit", help: "alias for /stop"},
		"list": {fn: cmdListBlocks, usage: "/list", help: "list the first 50 block ids"},
		"pos":  {fn: cmdShowPos, usage: "/pos", help: "show the player's current position"},
		"clear": {
			fn:    cmdClear,
			usage: "/clear",
			help:  "clear the terminal output buffer",
		},
	}
}

// dispatchCommand parses and runs line. The boolean indicates whether a
// handler was found (true for both successful runs and "unknown command"
// responses; false when the input was empty).
func dispatchCommand(game *Game, line string) (output string, ok bool) {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "/")
	if line == "" {
		return "", false
	}
	parts := strings.Fields(line)
	name := strings.ToLower(parts[0])
	args := parts[1:]
	if h, found := commands[name]; found {
		return h.fn(game, args), true
	}
	return fmt.Sprintf("unknown command: %s (try /help)", name), true
}

func cmdHelp(game *Game, args []string) string {
	if len(args) == 0 {
		names := make([]string, 0, len(commands))
		seen := map[string]bool{}
		for n, h := range commands {
			if seen[h.usage] {
				continue
			}
			seen[h.usage] = true
			names = append(names, n)
		}
		sort.Strings(names)
		var b strings.Builder
		b.WriteString("commands: ")
		for i, n := range names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(n)
		}
		b.WriteString("\ntype /help <cmd> for usage")
		return b.String()
	}
	sub := strings.ToLower(strings.TrimPrefix(args[0], "/"))
	h, ok := commands[sub]
	if !ok {
		return fmt.Sprintf("unknown command: %s", sub)
	}
	return fmt.Sprintf("%s\n%s", h.usage, h.help)
}

func cmdTeleport(game *Game, args []string) string {
	if len(args) != 3 {
		return "usage: /tp <x> <y> <z>"
	}
	x, ex := strconv.ParseFloat(args[0], 32)
	y, ey := strconv.ParseFloat(args[1], 32)
	z, ez := strconv.ParseFloat(args[2], 32)
	if ex != nil || ey != nil || ez != nil {
		return "tp: x, y, z must be numbers"
	}
	// TeleportPlayer takes game.mu itself; release our hold briefly to avoid
	// a double-lock deadlock. Callers invoke dispatchCommand under mu.
	game.mu.Unlock()
	game.TeleportPlayer(float32(x), float32(y), float32(z))
	game.mu.Lock()
	return fmt.Sprintf("teleported to (%.1f, %.1f, %.1f)", x, y, z)
}

func cmdSetBlock(game *Game, args []string) string {
	if len(args) != 4 {
		return "usage: /sb <x> <y> <z> <id>"
	}
	x, ex := strconv.Atoi(args[0])
	y, ey := strconv.Atoi(args[1])
	z, ez := strconv.Atoi(args[2])
	id, ei := strconv.Atoi(args[3])
	if ex != nil || ey != nil || ez != nil || ei != nil {
		return "sb: x, y, z, id must be integers"
	}
	if id < 0 || id >= BlockCount() {
		return fmt.Sprintf("sb: id out of range (0..%d)", BlockCount()-1)
	}
	// SetBlockAt locks internally; setBlockAtLocked does not.
	game.setBlockAtLocked(x, y, z, uint8(id))
	name := "?"
	if bi := Block(uint8(id)); bi != nil {
		name = bi.Name
	}
	return fmt.Sprintf("placed %s (id %d) at (%d, %d, %d)", name, id, x, y, z)
}

func cmdGamemode(game *Game, args []string) string {
	if len(args) != 1 {
		return "usage: /gm <survival|creative|spectator>"
	}
	var mode typhoon.Gamemode
	var label string
	switch strings.ToLower(args[0]) {
	case "s", "survival", "0":
		mode = typhoon.SURVIVAL
		label = "Survival"
	case "c", "creative", "1":
		mode = typhoon.CREATIVE
		label = "Creative"
	case "sp", "spectator", "3":
		mode = typhoon.SPECTATOR
		label = "Spectator"
	default:
		return fmt.Sprintf("gm: unknown mode %q", args[0])
	}
	// The caller already holds game.mu; assign directly to avoid deadlocking
	// on SetGamemode.
	game.player.gamemode = mode
	return "gamemode: " + label
}

func cmdStop(game *Game, args []string) string {
	if w := game.win; w != nil {
		w.SetShouldClose(true)
	}
	return "bye"
}

func cmdListBlocks(game *Game, args []string) string {
	n := BlockCount()
	if n == 0 {
		return "no blocks loaded"
	}
	limit := n
	if limit > 50 {
		limit = 50
	}
	var b strings.Builder
	for i := 0; i < limit; i++ {
		bi := Block(uint8(i))
		name := "?"
		if bi != nil {
			name = bi.Name
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%3d  %s", i, name)
	}
	if n > limit {
		fmt.Fprintf(&b, "\n... (%d more)", n-limit)
	}
	return b.String()
}

func cmdShowPos(game *Game, args []string) string {
	p := game.player.pos
	return fmt.Sprintf("pos: (%.2f, %.2f, %.2f)", p.x, p.y, p.z)
}

func cmdClear(game *Game, args []string) string {
	if game.terminal != nil {
		game.terminal.output = game.terminal.output[:0]
	}
	return ""
}
