package game

import (
	typhoon "github.com/TyphoonMC/TyphoonCore"
)

const (
	gravityAccel   = 0.08
	maxFallSpeed   = 3.0
	jumpImpulse    = 0.44
	walkSpeed      = 0.22
	airControlMult = 0.35 // acceleration multiplier when airborne
	groundFriction = 0.6
	airFriction    = 0.91
	playerWidth    = 0.6
	playerHeight   = 1.75
	collisionEps   = 1e-4
)

// playerAABB returns the world-space axis-aligned bounding box for a player
// whose feet sit at p. The box is centred on p.x / p.z horizontally.
func playerAABB(p FPoint3D) AABB {
	half := float32(playerWidth) / 2
	return AABB{
		MinX: p.x - half,
		MinY: p.y,
		MinZ: p.z - half,
		MaxX: p.x + half,
		MaxY: p.y + float32(playerHeight),
		MaxZ: p.z + half,
	}
}

// solidBoxesInRange collects every world-space solid block sub-AABB whose
// block cell falls inside [minX..maxX] x [minY..maxY] x [minZ..maxZ]
// (inclusive cell indices).
func (game *Game) solidBoxesInRange(minX, minY, minZ, maxX, maxY, maxZ int, out []AABB) []AABB {
	if minY < 0 {
		minY = 0
	}
	if maxY >= worldHeight {
		maxY = worldHeight - 1
	}
	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			for z := minZ; z <= maxZ; z++ {
				id := game.getBlockAt(x, y, z)
				if id == 0 {
					continue
				}
				info := Block(id)
				if info == nil || !info.Solid {
					continue
				}
				boxes := info.CollisionBoxes
				if len(boxes) == 0 {
					// Fallback: treat solid-without-boxes as a full cube.
					boxes = []AABB{{0, 0, 0, 1, 1, 1}}
				}
				fx := float32(x)
				fy := float32(y)
				fz := float32(z)
				for _, b := range boxes {
					out = append(out, AABB{
						MinX: fx + b.MinX,
						MinY: fy + b.MinY,
						MinZ: fz + b.MinZ,
						MaxX: fx + b.MaxX,
						MaxY: fy + b.MaxY,
						MaxZ: fz + b.MaxZ,
					})
				}
			}
		}
	}
	return out
}

// moveAxisAndCollide sweeps the player AABB along a single axis by delta,
// clipping against solid block sub-AABBs and updating player.pos in place.
// axis: 0=X, 1=Y, 2=Z. Returns the actually-applied delta and whether the
// sweep was clipped by a collider.
func (game *Game) moveAxisAndCollide(axis int, delta float32) (float32, bool) {
	if delta == 0 {
		return 0, false
	}

	box := playerAABB(game.player.pos)

	// Swept AABB covering source and destination.
	swept := box
	switch axis {
	case 0:
		if delta > 0 {
			swept.MaxX += delta
		} else {
			swept.MinX += delta
		}
	case 1:
		if delta > 0 {
			swept.MaxY += delta
		} else {
			swept.MinY += delta
		}
	case 2:
		if delta > 0 {
			swept.MaxZ += delta
		} else {
			swept.MinZ += delta
		}
	}

	// Expand 1 voxel on each side to catch edge-case grazing boxes.
	minX := floorInt(swept.MinX - 1)
	minY := floorInt(swept.MinY - 1)
	minZ := floorInt(swept.MinZ - 1)
	maxX := floorInt(swept.MaxX + 1)
	maxY := floorInt(swept.MaxY + 1)
	maxZ := floorInt(swept.MaxZ + 1)

	candidates := game.solidBoxesInRange(minX, minY, minZ, maxX, maxY, maxZ, nil)

	clipped := delta
	for _, c := range candidates {
		switch axis {
		case 0:
			// Overlap on Y/Z required.
			if box.MinY >= c.MaxY || box.MaxY <= c.MinY {
				continue
			}
			if box.MinZ >= c.MaxZ || box.MaxZ <= c.MinZ {
				continue
			}
			if clipped > 0 && box.MaxX <= c.MinX {
				gap := c.MinX - box.MaxX - collisionEps
				if gap < clipped {
					if gap < 0 {
						gap = 0
					}
					clipped = gap
				}
			} else if clipped < 0 && box.MinX >= c.MaxX {
				gap := c.MaxX - box.MinX + collisionEps
				if gap > clipped {
					if gap > 0 {
						gap = 0
					}
					clipped = gap
				}
			}
		case 1:
			if box.MinX >= c.MaxX || box.MaxX <= c.MinX {
				continue
			}
			if box.MinZ >= c.MaxZ || box.MaxZ <= c.MinZ {
				continue
			}
			if clipped > 0 && box.MaxY <= c.MinY {
				gap := c.MinY - box.MaxY - collisionEps
				if gap < clipped {
					if gap < 0 {
						gap = 0
					}
					clipped = gap
				}
			} else if clipped < 0 && box.MinY >= c.MaxY {
				gap := c.MaxY - box.MinY + collisionEps
				if gap > clipped {
					if gap > 0 {
						gap = 0
					}
					clipped = gap
				}
			}
		case 2:
			if box.MinX >= c.MaxX || box.MaxX <= c.MinX {
				continue
			}
			if box.MinY >= c.MaxY || box.MaxY <= c.MinY {
				continue
			}
			if clipped > 0 && box.MaxZ <= c.MinZ {
				gap := c.MinZ - box.MaxZ - collisionEps
				if gap < clipped {
					if gap < 0 {
						gap = 0
					}
					clipped = gap
				}
			} else if clipped < 0 && box.MinZ >= c.MaxZ {
				gap := c.MaxZ - box.MinZ + collisionEps
				if gap > clipped {
					if gap > 0 {
						gap = 0
					}
					clipped = gap
				}
			}
		}
	}

	switch axis {
	case 0:
		game.player.pos.x += clipped
	case 1:
		game.player.pos.y += clipped
	case 2:
		game.player.pos.z += clipped
	}

	collided := abs32(clipped-delta) > collisionEps
	return clipped, collided
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func lerp32(a, b, t float32) float32 {
	return a + (b-a)*t
}

// updatePhysics advances the player one frame in survival mode: horizontal
// acceleration toward the input wish vector, gravity, jump impulse, then
// per-axis swept collision against all solid block AABBs.
func (game *Game) updatePhysics() {
	if game.player.gamemode == typhoon.CREATIVE ||
		game.player.gamemode == typhoon.SPECTATOR {
		// Creative / spectator use direct position moves in input loop.
		return
	}

	wishX := game.player.wishDirX * walkSpeed
	wishZ := game.player.wishDirZ * walkSpeed

	// Blend horizontal velocity toward wish vector: heavier ground control,
	// lighter air control. Friction below handles the residual decay when
	// wish == 0.
	var blend float32
	if game.player.onGround {
		blend = 1 - groundFriction // 0.4
	} else {
		blend = airControlMult * 0.3 // ~0.1 effective air control
	}
	game.player.velocity.x = lerp32(game.player.velocity.x, wishX, blend)
	game.player.velocity.z = lerp32(game.player.velocity.z, wishZ, blend)

	// Light friction so we don't drift forever when wishDir is zero.
	if game.player.onGround {
		game.player.velocity.x *= groundFriction
		game.player.velocity.z *= groundFriction
	} else {
		game.player.velocity.x *= airFriction
		game.player.velocity.z *= airFriction
	}

	// Gravity.
	game.player.velocity.y -= gravityAccel
	if game.player.velocity.y < -maxFallSpeed {
		game.player.velocity.y = -maxFallSpeed
	}

	// Jump: consume the request only when standing on something.
	if game.player.wantsJump && game.player.onGround {
		game.player.velocity.y = jumpImpulse
		game.player.onGround = false
	}

	// Move per axis, applying collision each time.
	game.moveAxisAndCollide(0, game.player.velocity.x)
	_, collY := game.moveAxisAndCollide(1, game.player.velocity.y)
	game.moveAxisAndCollide(2, game.player.velocity.z)

	if collY {
		if game.player.velocity.y < 0 {
			game.player.onGround = true
		}
		// Either hit floor or ceiling -> cancel vertical velocity.
		game.player.velocity.y = 0
	} else {
		game.player.onGround = false
	}

	// Safety: falling below the world should not trap the player in negative
	// infinity. Clamp feet to 0 and treat as ground.
	if game.player.pos.y < 0 {
		game.player.pos.y = 0
		game.player.velocity.y = 0
		game.player.onGround = true
	}

	chkX := floorInt(game.player.pos.x) >> 4
	chkY := floorInt(game.player.pos.z) >> 4
	if chkX != game.middle.x || chkY != game.middle.y {
		game.newMiddle(Point2D{chkX, chkY})
	}
}
