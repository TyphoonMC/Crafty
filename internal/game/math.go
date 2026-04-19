package game

import (
	"math"

	"github.com/chewxy/math32"
	"github.com/pkg/errors"
)

type Point2D struct {
	x, y int
}

type Point3D struct {
	x, y, z int
}

type FPoint2D struct {
	x, y float32
}

type FPoint3D struct {
	x, y, z float32
}

func toRadian32(x float32) float32 {
	return x * math.Pi / 180
}

func toRadian64(x float64) float64 {
	return x * math.Pi / 180
}

// floorInt returns the largest int <= f. Unlike int(f) which truncates toward
// zero, this floors toward -infinity so negative world coordinates map to the
// correct voxel cell (e.g. -0.5 -> -1, not 0).
func floorInt(f float32) int {
	i := int(f)
	if float32(i) > f {
		return i - 1
	}
	return i
}

func sign32(v float32) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}

func FtoPoint3D(p *FPoint3D) *Point3D {
	return &Point3D{floorInt(p.x), floorInt(p.y), floorInt(p.z)}
}

func toFPoint3D(p *Point3D) *FPoint3D {
	return &FPoint3D{float32(p.x), float32(p.y), float32(p.z)}
}

func FMultiply(p *FPoint3D, t float32) {
	p.x *= t
	p.y *= t
	p.z *= t
}

// getPlayerBlockInSight performs a step-based raycast from the player's head
// along the view direction, returning the first solid block hit and the face
// normal (pointing from the solid block back to the empty cell the ray came
// from). The face is a unit vector along a single axis, suitable for
// placing a new block adjacent to the hit block.
func (game *Game) getPlayerBlockInSight(maxDist int) (*Point3D, *Point3D, error) {
	yaw := toRadian32(-game.player.rot.y + 180)
	pitch := toRadian32(game.player.rot.x)
	cosPitch := math32.Cos(pitch)
	dx := math32.Sin(yaw) * cosPitch
	dy := -math32.Sin(pitch)
	dz := math32.Cos(yaw) * cosPitch

	length := math32.Sqrt(dx*dx + dy*dy + dz*dz)
	if length < 1e-6 {
		return nil, nil, errors.New("invalid direction")
	}
	dx /= length
	dy /= length
	dz /= length

	origin := FPoint3D{game.player.pos.x, game.player.pos.y + 1.5, game.player.pos.z}

	const step = float32(0.05)
	steps := int(float32(maxDist) / step)

	prev := FtoPoint3D(&origin)
	for i := 1; i <= steps; i++ {
		t := step * float32(i)
		p := FPoint3D{origin.x + dx*t, origin.y + dy*t, origin.z + dz*t}
		cell := FtoPoint3D(&p)
		if cell.x == prev.x && cell.y == prev.y && cell.z == prev.z {
			continue
		}
		if game.getBlockAt(cell.x, cell.y, cell.z) != 0 {
			face := Point3D{}
			switch {
			case cell.x != prev.x:
				face.x = prev.x - cell.x
			case cell.y != prev.y:
				face.y = prev.y - cell.y
			case cell.z != prev.z:
				face.z = prev.z - cell.z
			}
			return cell, &face, nil
		}
		prev = cell
	}
	return nil, nil, errors.New("too far line of sight")
}
