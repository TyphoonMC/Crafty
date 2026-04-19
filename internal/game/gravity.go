package game

import typhoon "github.com/TyphoonMC/TyphoonCore"

const Gravity = .3

func (game *Game) calculateGravity() {
	if game.player.gamemode == typhoon.CREATIVE ||
		game.player.gamemode == typhoon.SPECTATOR {
		return
	}

	if game.isOnGround(&game.player.pos) {
		return
	}
	game.player.pos.y -= Gravity
}

// isOnGround returns true when a gravity step from p would push the player's
// feet into a solid block.
func (game *Game) isOnGround(p *FPoint3D) bool {
	x := floorInt(p.x)
	z := floorInt(p.z)
	belowCell := floorInt(p.y - Gravity)
	if belowCell < 0 {
		return true
	}
	return game.getBlockAt(x, belowCell, z) != 0
}
