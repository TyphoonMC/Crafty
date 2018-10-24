package main

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

func FtoPoint3D(p *FPoint3D) *Point3D {
	return &Point3D{int(p.x), int(p.y), int(p.z)}
}

func toFPoint3D(p *Point3D) *FPoint3D {
	return &FPoint3D{float32(p.x), float32(p.y), float32(p.z)}
}

func FMultiply(p *FPoint3D, t float32) {
	p.x *= t
	p.y *= t
	p.z *= t
}

func (game *Game) getPlayerBlockInSight(max int) (*Point3D, *Point3D, error) {
	x := math32.Sin(toRadian32(-game.player.rot.y + 180))
	y := math32.Cos(toRadian32(-game.player.rot.x - 90))
	z := math32.Cos(toRadian32(-game.player.rot.y + 180))

	loc := FPoint3D{game.player.pos.x, game.player.pos.y + 1.5, game.player.pos.z}

	try := 0
	for game.getBlockAtF(&loc) == 0 && try <= max {
		loc.x += x
		loc.y += y
		loc.z += z
		try++
	}

	face := Point3D{}
	if x > y {
		if z > x {
			face.z = 1
		} else {
			face.x = 1
		}
	} else {
		if z > y {
			face.z = 1
		} else {
			face.y = 1
		}
	}

	if try > max {
		return FtoPoint3D(&loc), &face, errors.New("too far line of sight")
	}
	return FtoPoint3D(&loc), &face, nil
}