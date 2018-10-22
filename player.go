package main

import "github.com/TyphoonMC/TyphoonCore"

type Player struct {
	pos         FPoint3D
	rot         FPoint3D
	speed       float32
	cameraSpeed float32
	velocity	FPoint3D
	gamemode    typhoon.Gamemode
}

func newPlayer() *Player {
	return &Player{
		FPoint3D{0, 3, 0},
		FPoint3D{0, 0, 0},
		0.2,
		0.2,
		FPoint3D{0, 0, 0},
		typhoon.SURVIVAL,
	}
}

func (game *Game) calculateVelocity() {
	game.player.pos.x += game.player.velocity.x
	game.player.pos.y += game.player.velocity.y
	game.player.pos.z += game.player.velocity.z
	FMultiply(&game.player.velocity, .9)
}