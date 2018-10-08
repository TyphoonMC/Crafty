package main

import "github.com/go-gl/glfw/v3.2/glfw"

type Chunk struct {
	coordinates Point2D
	blocks      [16][128][16]uint8
}

type Game struct {
	win   *glfw.Window
	focus bool

	player *Player

	chunks []*Chunk
}

func newGame() *Game {
	g := Game{
		nil,
		false,
		newPlayer(),
		nil,
	}

	g.loadChunks()

	return &g
}

func (game *Game) loadChunks() {
	game.loadChunk(Point2D{-1, -1})
	game.loadChunk(Point2D{-1, 0})
	game.loadChunk(Point2D{0, -1})
	game.loadChunk(Point2D{0, 0})
	game.loadChunk(Point2D{1, 0})
	game.loadChunk(Point2D{0, 1})
	game.loadChunk(Point2D{1, 1})
}

func (game *Game) setBlockAt(x, y, z int, id uint8) {
	chunk := Point2D{x / 16, z / 16}
	b := Point3D{x - chunk.x, y, z - chunk.y}

	var chk *Chunk = nil
	for _, c := range game.chunks {
		if c.coordinates.x == chunk.x && c.coordinates.y == chunk.y {
			chk = c
			break
		}
	}

	if chk == nil {
		panic("chunk not found")
	}

	chk.blocks[b.x][b.y][b.z] = id
}

func (game *Game) loadChunk(coordinate Point2D) {
	c := Chunk{coordinates: coordinate}
	for x, a := range c.blocks {
		for y, b := range a {
			for z := range b {
				if y > 3 {
					c.blocks[x][y][z] = 0
				} else {
					c.blocks[x][y][z] = uint8(y)
				}
			}
		}
	}
	game.chunks = append(game.chunks, &c)
}
