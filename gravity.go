package main

import "github.com/TyphoonMC/TyphoonCore"

const(
	Gravity = .3
)

func (game *Game) calculateGravity() {
	if game.player.gamemode == typhoon.CREATIVE ||
		game.player.gamemode == typhoon.SPECTATOR {
			return
	}

	ground := game.isOnGround(&game.player.pos)
	if !ground {
		game.player.pos.y -= Gravity
	}
}

func (game *Game) isOnGround(p *FPoint3D) bool {
	loc := FtoPoint3D(p)
	next := FPoint3D{p.x, p.y-Gravity, p.z}
	nextLoc := FtoPoint3D(&next)

	if nextLoc.y != loc.y {
		block := game.getBlockAt(loc.x, loc.y-1, loc.z)
		return block != 0
	}
	return false
}