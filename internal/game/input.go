package game

import (
	"log"
	"math"

	typhoon "github.com/TyphoonMC/TyphoonCore"
	"github.com/go-gl/glfw/v3.3/glfw"
)

func (game *Game) setFocused() {
	game.focus = true
	game.win.SetInputMode(glfw.CursorMode, glfw.CursorHidden)
}

func (game *Game) setUnfocused() {
	game.focus = false
	game.win.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
}

func (game *Game) keysCallback(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	// Accept both Press and Repeat so holding Backspace or an arrow key
	// continues to mutate the edit buffer while the terminal is open.
	if action == glfw.Release {
		return
	}
	game.mu.Lock()
	defer game.mu.Unlock()

	if game.terminal != nil && game.terminal.IsOpen() {
		game.handleTerminalKey(key, action, mods)
		return
	}

	if action != glfw.Press {
		return
	}

	if key == glfw.KeyEscape {
		game.setUnfocused()
		return
	}

	// Open the terminal on T or /. The matching char event fires right
	// after — set swallowNextChar so we don't echo the trigger key into
	// the input buffer.
	if key == glfw.KeyT || key == glfw.KeySlash {
		if game.terminal != nil {
			game.terminal.Open()
			game.terminal.swallowNextChar = true
		}
		game.setUnfocused()
	}
}

// handleTerminalKey processes editor-level key events (Enter, Esc, arrows,
// backspace, history). Printable characters are handled by charCallback.
func (game *Game) handleTerminalKey(key glfw.Key, action glfw.Action, mods glfw.ModifierKey) {
	t := game.terminal
	switch key {
	case glfw.KeyEscape:
		t.Close()
		game.setUnfocused()
	case glfw.KeyEnter, glfw.KeyKPEnter:
		line := t.Commit()
		if line != "" {
			t.AddOutput("> "+line, RGBA{180, 170, 200, 255})
			out, handled := dispatchCommand(game, line)
			if handled && out != "" {
				t.AddOutput(out, RGBA{220, 240, 200, 255})
			}
		}
		t.Close()
	case glfw.KeyBackspace:
		t.Backspace()
	case glfw.KeyDelete:
		t.Delete()
	case glfw.KeyLeft:
		t.CursorLeft()
	case glfw.KeyRight:
		t.CursorRight()
	case glfw.KeyHome:
		t.CursorHome()
	case glfw.KeyEnd:
		t.CursorEnd()
	case glfw.KeyUp:
		t.HistoryUp()
	case glfw.KeyDown:
		t.HistoryDown()
	}
}

// charCallback receives decoded Unicode codepoints from GLFW. GLFW only
// emits printable characters here, so control keys (Enter, Backspace,
// arrows, Esc) are already filtered by the platform.
func (game *Game) charCallback(w *glfw.Window, char rune) {
	game.mu.Lock()
	defer game.mu.Unlock()
	if game.terminal == nil || !game.terminal.IsOpen() {
		return
	}
	if game.terminal.swallowNextChar {
		game.terminal.swallowNextChar = false
		return
	}
	if char >= 0x20 && char != 0x7F {
		game.terminal.InsertRune(char)
	}
}

func (game *Game) cursorCallback(w *glfw.Window, xpos float64, ypos float64) {
	game.mu.Lock()
	defer game.mu.Unlock()
	if !game.focus {
		return
	}
	if game.terminal != nil && game.terminal.IsOpen() {
		return
	}
	width, height := game.win.GetSize()
	cx := float64(width) / 2
	cy := float64(height) / 2

	posX, posY := game.win.GetCursorPos()
	game.player.rot.y += float32(posX-cx) * game.player.cameraSpeed

	rX := float32(posY-cy) * game.player.cameraSpeed
	nX := game.player.rot.x + rX
	if nX > -90 && nX < 90 {
		game.player.rot.x = nX
	}

	game.win.SetCursorPos(cx, cy)
}

func (game *Game) mouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mod glfw.ModifierKey) {
	game.mu.Lock()
	defer game.mu.Unlock()

	// While the terminal is open the mouse is the user's "how do I dismiss
	// this?" escape hatch — don't steal it to refocus gameplay or place a
	// block. Esc or Enter close the overlay.
	if game.terminal != nil && game.terminal.IsOpen() {
		return
	}

	if !game.focus {
		game.setFocused()
		return
	}
	if action != glfw.Press {
		return
	}

	loc, face, err := game.getPlayerBlockInSight(10)
	if err != nil {
		return
	}
	switch button {
	case glfw.MouseButtonLeft:
		// Break the block we're looking at.
		game.setBlockAtLocked(loc.x, loc.y, loc.z, 0)
	case glfw.MouseButtonRight:
		// Place a block on the face we're looking at (adjacent empty cell).
		game.setBlockAtLocked(loc.x+face.x, loc.y+face.y, loc.z+face.z, 4)
	}
}

func (game *Game) InitInput() {
	game.win.SetKeyCallback(game.keysCallback)
	game.win.SetCharCallback(game.charCallback)
	game.win.SetCursorPosCallback(game.cursorCallback)
	game.win.SetMouseButtonCallback(game.mouseButtonCallback)
}

func (game *Game) checkKey(key glfw.Key) bool {
	return game.win.GetKey(key) == 1
}

// TeleportPlayer safely teleports the player from another goroutine.
func (game *Game) TeleportPlayer(x, y, z float32) {
	game.mu.Lock()
	defer game.mu.Unlock()
	game.teleportPlayerLocked(x, y, z)
}

func (game *Game) teleportPlayerLocked(x, y, z float32) {
	game.player.pos.x = x
	game.player.pos.y = y
	game.player.pos.z = z
	game.player.velocity = FPoint3D{}
	game.player.onGround = false

	chkX := floorInt(game.player.pos.x) >> 4
	chkY := floorInt(game.player.pos.z) >> 4

	if chkX != game.middle.x || chkY != game.middle.y {
		game.newMiddle(Point2D{chkX, chkY})
	}
}

func (game *Game) inputLoop() {
	// Reset per-frame intent flags so released keys stop driving motion.
	game.player.wishDirX = 0
	game.player.wishDirZ = 0
	game.player.wantsJump = false

	// Terminal open — suppress movement intent so typing doesn't also walk
	// the player around. Look controls are already gated by game.focus.
	if game.terminal != nil && game.terminal.IsOpen() {
		return
	}

	var dForward, dRight float32
	if game.checkKey(glfw.KeyW) || game.checkKey(glfw.KeyUp) {
		dForward += 1
	}
	if game.checkKey(glfw.KeyS) || game.checkKey(glfw.KeyDown) {
		dForward -= 1
	}
	if game.checkKey(glfw.KeyD) || game.checkKey(glfw.KeyRight) {
		dRight += 1
	}
	if game.checkKey(glfw.KeyA) || game.checkKey(glfw.KeyLeft) {
		dRight -= 1
	}

	// Project input onto the XZ plane using player yaw. With rot.y=0 the
	// forward key should push along -Z (matching the view-matrix basis used
	// by buildView and the old movePlayer yaw math).
	yaw := float64(toRadian32(game.player.rot.y))
	sinY := float32(math.Sin(yaw))
	cosY := float32(math.Cos(yaw))
	wishX := sinY*dForward + cosY*dRight
	wishZ := -cosY*dForward + sinY*dRight
	mag := math.Sqrt(float64(wishX*wishX + wishZ*wishZ))
	if mag > 1 {
		wishX /= float32(mag)
		wishZ /= float32(mag)
	}
	game.player.wishDirX = wishX
	game.player.wishDirZ = wishZ

	if game.player.gamemode == typhoon.CREATIVE ||
		game.player.gamemode == typhoon.SPECTATOR {
		// Creative/spectator: direct position move, no physics.
		s := game.player.speed
		game.player.pos.x += wishX * s
		game.player.pos.z += wishZ * s
		if game.checkKey(glfw.KeySpace) {
			game.player.pos.y += s
		}
		if game.checkKey(glfw.KeyLeftShift) {
			game.player.pos.y -= s
		}

		chkX := floorInt(game.player.pos.x) >> 4
		chkY := floorInt(game.player.pos.z) >> 4
		if chkX != game.middle.x || chkY != game.middle.y {
			game.newMiddle(Point2D{chkX, chkY})
		}
	} else {
		game.player.wantsJump = game.checkKey(glfw.KeySpace)
	}

	if game.checkKey(glfw.KeyN) {
		loc := game.player.pos
		loc.y -= 1
		c, b := game.getChunkBlockAt(floorInt(loc.x), floorInt(loc.y), floorInt(loc.z))
		log.Println(game.player.onGround, game.getBlockAtF(&loc), c, b, game.player.pos, game.player.rot)
	}
}
