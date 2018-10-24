package main

import (
	"github.com/go-gl/glfw/v3.2/glfw"
	"math"
	"github.com/TyphoonMC/TyphoonCore"
	"log"
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

	if action == glfw.Press {
		loc, face, err := game.getPlayerBlockInSight(10)
		if err == nil {
			if button == glfw.MouseButtonLeft {
				game.setBlockAt(loc.x + face.x, loc.y + face.y, loc.z + face.z, 0)
			} else if button == glfw.MouseButtonRight {
				game.setBlockAt(loc.x + face.x, loc.y + face.y, loc.z + face.z, 4)
			}
		}
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

	nPos := FPoint3D{game.player.pos.x, game.player.pos.y, game.player.pos.z}

	nPos.x += float32(x) * game.player.speed
	nPos.z += float32(y) * game.player.speed

	nPosInt := FtoPoint3D(&nPos)
	posInt := FtoPoint3D(&game.player.pos)

	if game.getBlockAt(nPosInt.x, nPosInt.y, nPosInt.z) != 0 &&
		game.player.gamemode != typhoon.SPECTATOR {
		if nPosInt.x != posInt.x {
			nPos.x = game.player.pos.x
		}
		if nPosInt.z != posInt.z {
			nPos.z = game.player.pos.z
		}
	}

	game.player.pos = nPos

	chkX := int(game.player.pos.x) >> 4
	chkY := int(game.player.pos.z) >> 4

	if chkX != game.middle.x || chkY != game.middle.y {
		game.newMiddle(Point2D{chkX, chkY})
	}
}

func (game *Game) teleportPlayer(x, y, z float32) {
	game.player.pos.x = float32(x)
	game.player.pos.x = float32(y)
	game.player.pos.x = float32(z)

	chkX := int(game.player.pos.x) >> 4
	chkY := int(game.player.pos.z) >> 4

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

	if game.checkKey(glfw.KeyN) {
		loc := game.player.pos
		loc.y -= 1
		c, b := game.getChunkBlockAt(int(loc.x), int(loc.y), int(loc.z))
		log.Println(game.isOnGround(&game.player.pos), game.getBlockAtF(&loc), c, b, game.player.pos, game.player.rot)
	}

	if game.player.gamemode == typhoon.CREATIVE ||
		game.player.gamemode == typhoon.SPECTATOR {
		if game.checkKey(glfw.KeySpace) {
			game.player.pos.y += s
		}
		if game.checkKey(glfw.KeyLeftShift) {
			game.player.pos.y -= s
		}
	} else if game.checkKey(glfw.KeySpace) && game.isOnGround(&game.player.pos) {
		game.player.velocity = FPoint3D{0, .6, 0}
	}
}
