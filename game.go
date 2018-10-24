package main

import (
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/kandoo/beehive/gob"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"github.com/aquilax/go-perlin"
)

type Chunk struct {
	coordinates Point2D
	Blocks      [16][128][16]uint8
	mask 		[16][128][16]*FaceMask
}

type Game struct {
	win   *glfw.Window
	focus bool

	player *Player

	middle Point2D
	grid   [3][3]*Chunk

	generator *perlin.Perlin
}

func newGame() *Game {
	g := Game{
		nil,
		false,
		newPlayer(),
		Point2D{0, 0},
		[3][3]*Chunk{},
		nil,
	}

	g.initPerlin(189766828)
	g.loadChunks()

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
			game.saveChunk(game.grid[c.x-game.middle.x][c.y-game.middle.y])
		}
	}
}

func (game *Game) newMiddle(coord Point2D) {
	log.Println("new middle", coord)
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
			game.saveChunk(cache[i])
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
	chunk, b := game.getChunkBlockAt(x, y, z)

	chk := game.getChunk(chunk.x, chunk.y, true)

	if chk == nil {
		panic("chunk not found")
	}

	log.Println("setblock", chunk, b, x, y, z)
	chk.Blocks[b.x][b.y][b.z] = id
}

func (game *Game) setBlockAtF(d *FPoint3D, id uint8) {
	game.setBlockAt(int(d.x), int(d.y), int(d.z), id)
}

func (game *Game) getChunkBlockAt(x, y, z int) (*Point2D, *Point3D) {
	chunk := Point2D{x >> 4, z >> 4}
	b := Point3D{x & 15, y, z & 15}
	return &chunk, &b
}

func (game *Game) getBlockCoord(c *Point2D, b *Point3D) (*Point3D) {
	d := Point3D{c.x << 4 + b.x, b.y, c.y << 4 + + b.z}
	return &d
}

func (game *Game) getBlockAt(x, y, z int) uint8 {
	if y < 0 || y > 127 {
		return 0
	}

	chunk, b := game.getChunkBlockAt(x, y, z)

	chk := game.getChunk(chunk.x, chunk.y, false)

	if chk == nil {
		return 0
	}

	return chk.Blocks[b.x][b.y][b.z]
}

func (game *Game) getBlockAtF(d *FPoint3D) uint8 {
	return game.getBlockAt(int(d.x), int(d.y), int(d.z))
}

func (game *Game) loadChunk(coordinate Point2D) *Chunk {
	log.Println("loading chunk", coordinate)
	c := Chunk{coordinates: coordinate}

	if _, err := os.Stat(c.getFile()); os.IsNotExist(err) {
		log.Println("Generating chunk...")
		b := Point3D{}
		for b.x = 0; b.x < 16; b.x++ {
			for b.z = 0; b.z < 16; b.z++ {
				//coord := game.getBlockCoord(&c.coordinates, &b)
				//high := game.getHigh(coord.x, coord.z)
				for y := 0; y <= 3; y++ {
					if y < 3 {
						c.Blocks[b.x][y][b.z] = 1
					} else {
						c.Blocks[b.x][y][b.z] = 3
					}
				}
			}
		}
		game.saveChunk(&c)
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

func (game *Game) saveChunk(chunk *Chunk) {
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
