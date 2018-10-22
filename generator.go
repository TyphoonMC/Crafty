package main

import (
	"github.com/aquilax/go-perlin"
)

var (
	alpha = 2.
	beta = 2.
	n = 3
)

func (game *Game) initPerlin(seed int64) {
	game.generator = perlin.NewPerlin(alpha, beta, n, seed)
}

func (game *Game) getHigh(x, z int) int {
	rx := float64(x)/1000
	rz := float64(z)/1000
	h := int(game.generator.Noise2D(rx, rz)*100)
	if h < 1 {
		return 1
	}
	return h
}