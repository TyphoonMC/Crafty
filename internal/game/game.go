package game

import (
	"bytes"
	"encoding/gob"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	typhoon "github.com/TyphoonMC/TyphoonCore"
	"github.com/go-gl/glfw/v3.3/glfw"
)

const (
	chunksDir   = "./map/chunks"
	worldHeight = 128
)

type Chunk struct {
	Coordinates Point2D
	Blocks      [16][128][16]uint8
	// SkyLight: 0..15 per voxel, derived from top-down propagation. Not
	// serialized to disk — recomputed on chunk load.
	SkyLight [16][128][16]uint8
	// BlockLight: packed RGB each channel 0..15 in nibbles
	// (R<<8 | G<<4 | B). Not serialized to disk.
	BlockLight [16][128][16]uint16
}

type Game struct {
	mu sync.Mutex

	win   *glfw.Window
	focus bool

	player *Player

	middle Point2D
	grid   [3][3]*Chunk

	gen *terrainGen

	renderer *renderer
}

func NewGame() *Game {
	g := Game{
		player: newPlayer(),
		middle: Point2D{0, 0},
	}

	g.gen = newTerrainGen(189766828)
	g.loadChunks()

	return &g
}

func (game *Game) SetWindow(w *glfw.Window) {
	game.win = w
}

func (game *Game) Window() *glfw.Window {
	return game.win
}

func (game *Game) Player() *Player {
	return game.player
}

// SetGamemode safely updates the player's gamemode from another goroutine.
func (game *Game) SetGamemode(gm typhoon.Gamemode) {
	game.mu.Lock()
	defer game.mu.Unlock()
	game.player.gamemode = gm
}

func (game *Game) loadChunks() {
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			game.grid[x][y] = game.loadChunk(Point2D{game.middle.x + x - 1, game.middle.y + y - 1})
		}
	}
}

// UnloadChunks flushes all in-memory chunks to disk. Safe to call from the
// main thread at shutdown.
func (game *Game) UnloadChunks() {
	game.mu.Lock()
	defer game.mu.Unlock()
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			if game.grid[x][y] != nil {
				game.saveChunk(game.grid[x][y])
			}
		}
	}
}

func (game *Game) newMiddle(coord Point2D) {
	log.Println("new middle", coord)
	game.middle = coord

	cache := make([]*Chunk, 0, 9)
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			cache = append(cache, game.grid[x][y])
		}
	}

	reused := make([]bool, len(cache))

	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			wantX := game.middle.x + x - 1
			wantY := game.middle.y + y - 1
			index, chk := game.getChunkIn(wantX, wantY, cache)
			if chk != nil {
				game.grid[x][y] = chk
				reused[index] = true
			} else {
				game.grid[x][y] = game.loadChunk(Point2D{wantX, wantY})
			}
		}
	}

	for i, used := range reused {
		if !used && cache[i] != nil {
			game.saveChunk(cache[i])
		}
	}
}

func (game *Game) getChunkIn(x, z int, cache []*Chunk) (int, *Chunk) {
	for i, c := range cache {
		if c == nil {
			continue
		}
		if c.Coordinates.x == x && c.Coordinates.y == z {
			return i, c
		}
	}
	return -1, nil
}

func (game *Game) getChunk(x, z int, force bool) *Chunk {
	if x >= game.middle.x-1 && x <= game.middle.x+1 &&
		z >= game.middle.y-1 && z <= game.middle.y+1 {
		return game.grid[x-game.middle.x+1][z-game.middle.y+1]
	}
	if force {
		return game.loadChunk(Point2D{x, z})
	}
	return nil
}

// SetBlockAt safely sets a block from another goroutine (e.g. the admin
// server). The update is serialized against the main loop.
func (game *Game) SetBlockAt(x, y, z int, id uint8) {
	game.mu.Lock()
	defer game.mu.Unlock()
	game.setBlockAtLocked(x, y, z, id)
}

func (game *Game) setBlockAtLocked(x, y, z int, id uint8) {
	if y < 0 || y >= worldHeight {
		return
	}
	chunk, b := game.getChunkBlockAt(x, y, z)
	chk := game.getChunk(chunk.x, chunk.y, true)
	if chk == nil {
		return
	}
	chk.Blocks[b.x][b.y][b.z] = id

	// Recompute lighting for the edited chunk from scratch. v1 does not do
	// incremental relighting: it's fast enough for one chunk (~32k voxels).
	game.propagateChunkLight(chk)

	// Boundary edits also invalidate light on neighbour chunks (their inward
	// BFS may pick up light from this chunk's new state, and vice versa).
	neighbourChunks := make([]*Point2D, 0, 4)
	if b.x == 0 {
		n := Point2D{chunk.x - 1, chunk.y}
		neighbourChunks = append(neighbourChunks, &n)
	} else if b.x == 15 {
		n := Point2D{chunk.x + 1, chunk.y}
		neighbourChunks = append(neighbourChunks, &n)
	}
	if b.z == 0 {
		n := Point2D{chunk.x, chunk.y - 1}
		neighbourChunks = append(neighbourChunks, &n)
	} else if b.z == 15 {
		n := Point2D{chunk.x, chunk.y + 1}
		neighbourChunks = append(neighbourChunks, &n)
	}
	for _, nc := range neighbourChunks {
		if nchk := game.getChunk(nc.x, nc.y, false); nchk != nil {
			game.propagateChunkLight(nchk)
		}
	}

	if game.renderer != nil {
		game.renderer.markDirty(*chunk)
		for _, nc := range neighbourChunks {
			game.renderer.markDirty(*nc)
		}
	}
}

func (game *Game) setBlockAtF(d *FPoint3D, id uint8) {
	game.setBlockAtLocked(floorInt(d.x), floorInt(d.y), floorInt(d.z), id)
}

func (game *Game) getChunkBlockAt(x, y, z int) (*Point2D, *Point3D) {
	chunk := Point2D{x >> 4, z >> 4}
	b := Point3D{x & 15, y, z & 15}
	return &chunk, &b
}

func (game *Game) getBlockCoord(c *Point2D, b *Point3D) *Point3D {
	d := Point3D{(c.x << 4) + b.x, b.y, (c.y << 4) + b.z}
	return &d
}

func (game *Game) getBlockAt(x, y, z int) uint8 {
	if y < 0 || y >= worldHeight {
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
	return game.getBlockAt(floorInt(d.x), floorInt(d.y), floorInt(d.z))
}

func (game *Game) loadChunk(coordinate Point2D) *Chunk {
	log.Println("loading chunk", coordinate)
	c := Chunk{Coordinates: coordinate}

	if _, err := os.Stat(c.getFile()); os.IsNotExist(err) {
		log.Println("generating chunk", coordinate)
		game.gen.generateChunk(&c)
		game.saveChunk(&c)
	} else {
		data, err := os.ReadFile(c.getFile())
		if err != nil {
			panic(err)
		}
		dec := gob.NewDecoder(bytes.NewReader(data))
		if err := dec.Decode(&c.Blocks); err != nil {
			panic(err)
		}
	}
	// Light data is not serialized — recompute from block data on every load.
	game.propagateChunkLight(&c)
	return &c
}

func (chunk *Chunk) getFile() string {
	return filepath.Join(chunksDir, strconv.Itoa(chunk.Coordinates.x), strconv.Itoa(chunk.Coordinates.y))
}

func (chunk *Chunk) getDirectory() string {
	return filepath.Join(chunksDir, strconv.Itoa(chunk.Coordinates.x))
}

func (game *Game) saveChunk(chunk *Chunk) {
	log.Println("unloading chunk", chunk.Coordinates)

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(&chunk.Blocks); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(chunk.getDirectory(), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(chunk.getFile(), buf.Bytes(), 0o644); err != nil {
		panic(err)
	}
}
