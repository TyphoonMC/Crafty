package game

import typhoon "github.com/TyphoonMC/TyphoonCore"

type Player struct {
	pos         FPoint3D
	rot         FPoint3D
	speed       float32
	cameraSpeed float32
	velocity    FPoint3D
	gamemode    typhoon.Gamemode
}

func newPlayer() *Player {
	return &Player{
		pos:         FPoint3D{0, 110, 0},
		rot:         FPoint3D{0, 0, 0},
		speed:       0.2,
		cameraSpeed: 0.2,
		velocity:    FPoint3D{0, 0, 0},
		gamemode:    typhoon.SURVIVAL,
	}
}

func (game *Game) calculateVelocity() {
	game.player.pos.x += game.player.velocity.x
	game.player.pos.y += game.player.velocity.y
	game.player.pos.z += game.player.velocity.z
	FMultiply(&game.player.velocity, .9)
}
