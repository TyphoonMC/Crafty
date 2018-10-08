package main

import (
	"github.com/go-gl/glfw/v3.2/glfw"
	"math"
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
	switch key {
	case glfw.KeyEscape:
		game.setUnfocused()
		break
	}
}

func (game *Game) cursorCallback(w *glfw.Window, xpos float64, ypos float64) {
	if game.focus {
		w, h := game.win.GetSize()
		x := float64(w) / 2
		y := float64(h) / 2

		posX, posY := game.win.GetCursorPos()
		game.player.rot.y += float32(posX-x) * game.player.cameraSpeed

		// No screen reverse
		rX := float32(posY-y) * game.player.cameraSpeed
		nX := game.player.rot.x + rX
		if nX > -90 && nX < 90 {
			game.player.rot.x = nX
		}

		game.win.SetCursorPos(float64(w)/2, float64(h)/2)
	}
}

func (game *Game) mouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mod glfw.ModifierKey) {
	if !game.focus {
		game.setFocused()
	}
}

func (game *Game) initInput() {
	game.win.SetKeyCallback(game.keysCallback)
	game.win.SetCursorPosCallback(game.cursorCallback)
	game.win.SetMouseButtonCallback(game.mouseButtonCallback)
}

func (game *Game) checkKey(key glfw.Key) bool {
	return game.win.GetKey(key) == 1
}

func (game *Game) movePlayer(rot float32) {
	x := math.Sin(float64(toRadian32(-game.player.rot.y + rot)))
	y := math.Cos(float64(toRadian32(-game.player.rot.y + rot)))
	game.player.pos.x += float32(x) * game.player.speed
	game.player.pos.z += float32(y) * game.player.speed

	chkX := int(game.player.pos.x / 16)
	chkY := int(game.player.pos.z / 16)

	if chkX != game.middle.x || chkY != game.middle.y {
		game.newMiddle(Point2D{chkX, chkY})
	}
}

func (game *Game) inputLoop() {
	s := game.player.speed
	if game.checkKey(glfw.KeyUp) || game.checkKey(glfw.KeyW) {
		game.movePlayer(180)
	}
	if game.checkKey(glfw.KeyDown) || game.checkKey(glfw.KeyS) {
		game.movePlayer(0)
	}
	if game.checkKey(glfw.KeyLeft) || game.checkKey(glfw.KeyA) {
		game.movePlayer(-90)
	}
	if game.checkKey(glfw.KeyRight) || game.checkKey(glfw.KeyD) {
		game.movePlayer(90)
	}
	if game.checkKey(glfw.KeySpace) {
		game.player.pos.y += s
	}
	if game.checkKey(glfw.KeyLeftShift) {
		game.player.pos.y -= s
	}
}
