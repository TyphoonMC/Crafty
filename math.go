package main

import "math"

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

func FMultiply(p *FPoint3D, t float32) {
	p.x *= t
	p.y *= t
	p.z *= t
}