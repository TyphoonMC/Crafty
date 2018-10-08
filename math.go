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