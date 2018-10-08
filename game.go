package main

import (
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/kandoo/beehive/gob"
	"io/ioutil"
	"log"
	"os"
	"strconv"
)

type Chunk struct {
	coordinates Point2D
	Blocks      [16][128][16]uint8
}

type Game struct {
	win   *glfw.Window
	focus bool

	player *Player

	middle Point2D
	grid   [3][3]*Chunk
}

func newGame() *Game {
	g := Game{
		nil,
		false,
		newPlayer(),
		Point2D{0, 0},
		[3][3]*Chunk{},
	}

	go g.loadChunks()

	return &g
}

func (game *Game) loadChunks() {
	c := Point2D{}
	for c.x = 0; c.x < 3; c.x++ {
		for c.y = 0; c.y < 3; c.y++ {
			game.grid[c.x-game.middle.x][c.y-game.middle.y] = game.loadChunk(Point2D{c.x - 1, c.y - 1})
		}
	}
}

func (game *Game) unloadChunks() {
	c := Point2D{}
	for c.x = 0; c.x < 3; c.x++ {
		for c.y = 0; c.y < 3; c.y++ {
			game.unloadChunk(game.grid[c.x-game.middle.x][c.y-game.middle.y])
		}
	}
}

func (game *Game) newMiddle(coord Point2D) {
	game.middle = coord

	cache := make([]*Chunk, 0)

	c := Point2D{}
	for c.x = 0; c.x < 3; c.x++ {
		for c.y = 0; c.y < 3; c.y++ {
			cache = append(cache, game.grid[c.x][c.y])
		}
	}

	reused := make([]bool, len(cache))

	for c.x = 0; c.x < 3; c.x++ {
		for c.y = 0; c.y < 3; c.y++ {
			index, chk := game.getChunkIn(game.middle.x+c.x-1, game.middle.y+c.y-1, cache)
			if chk != nil {
				game.grid[c.x][c.y] = chk
				reused[index] = true
			} else {
				game.grid[c.x][c.y] = game.loadChunk(Point2D{game.middle.x + c.x - 1, game.middle.y + c.y - 1})
			}
		}
	}

	for i, j := range reused {
		if !j {
			game.unloadChunk(cache[i])
		}
	}
}

func (game *Game) getChunkIn(x, z int, cache []*Chunk) (int, *Chunk) {
	for i, c := range cache {
		if c.coordinates.x == x && c.coordinates.y == z {
			return i, c
		}
	}
	return -1, nil
}

func (game *Game) getChunk(x, z int, force bool) *Chunk {
	if x >= game.middle.x-1 && x <= game.middle.x+1 &&
		z >= game.middle.y-1 && z <= game.middle.y+1 {
		return game.grid[game.middle.x-x+1][game.middle.y-z+1]
	}

	if force {
		return game.loadChunk(Point2D{x, z})
	}
	return nil
}

func (game *Game) setBlockAt(x, y, z int, id uint8) {
	chunk := Point2D{x / 16, z / 16}
	b := Point3D{x - 16*chunk.x, y, z - 16*chunk.y}

	chk := game.getChunk(chunk.x, chunk.y, true)

	if chk == nil {
		panic("chunk not found")
	}

	chk.Blocks[b.x][b.y][b.z] = id
}

func (game *Game) getBlockAt(x, y, z int) uint8 {
	chunk := Point2D{x / 16, z / 16}
	b := Point3D{0, y, z - 0}
	if x >= 0 {
		b.x = x - 16*chunk.x
	} else {
		b.x = -(x - 16*chunk.x)
	}
	if z >= 0 {
		b.z = z - 16*chunk.y
	} else {
		b.z = -(z - 16*chunk.y)
	}

	chk := game.getChunk(chunk.x, chunk.y, false)

	if chk == nil {
		return 0
	}

	return chk.Blocks[b.x][b.y][b.z]
}

func (game *Game) loadChunk(coordinate Point2D) *Chunk {
	log.Println("loading chunk", coordinate)
	c := Chunk{coordinates: coordinate}

	if _, err := os.Stat(c.getFile()); os.IsNotExist(err) {
		for x, a := range c.Blocks {
			for y, b := range a {
				for z := range b {
					if y > 3 {
						c.Blocks[x][y][z] = 0
					} else {
						c.Blocks[x][y][z] = uint8(y)
					}
				}
			}
		}
	} else {
		data, err := ioutil.ReadFile(c.getFile())
		if err != nil {
			panic(err)
		}

		err = gob.Decode(&c, data)
		if err != nil {
			panic(err)
		}
	}
	return &c
}

func (chunk *Chunk) getFile() string {
	return "./map/chunks/" + strconv.Itoa(chunk.coordinates.x) + "/" + strconv.Itoa(chunk.coordinates.y)
}

func (chunk *Chunk) getDirectory() string {
	return "./map/chunks/" + strconv.Itoa(chunk.coordinates.x)
}

func (game *Game) unloadChunk(chunk *Chunk) {
	log.Println("unloading chunk", chunk.coordinates)

	data, err := gob.Encode(chunk)
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(chunk.getDirectory(), os.ModePerm)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(chunk.getFile(), data, os.ModePerm)
	if err != nil {
		panic(err)
	}
}
