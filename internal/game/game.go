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

	// middle is the chunk coordinate the player currently stands on. LOD 0
	// chunks load in a (2*lod0Radius+1)² square centred here; distant
	// surfaces fill out to lodMacroRadius.
	middle Point2D

	// chunks holds every LOD 0 chunk keyed by its chunk coordinate. A
	// lookup is O(1) and works for any radius without special-casing the
	// old 3x3 grid indices. Entries outside the LOD 0 square are
	// flushed and deleted by streamChunks.
	chunks map[Point2D]*Chunk

	// surfaces holds lightweight heightmap summaries for the distant-LOD
	// ring — chunks that are visible but not close enough to warrant
	// per-block mesh generation / physics. Keyed the same way as chunks.
	surfaces map[Point2D]*ChunkSurface

	gen *terrainGen

	renderer *renderer

	// terminal is the in-game console overlay (press T or / to open).
	terminal *Terminal
}

func NewGame() *Game {
	g := Game{
		player:   newPlayer(),
		middle:   Point2D{0, 0},
		chunks:   make(map[Point2D]*Chunk),
		surfaces: make(map[Point2D]*ChunkSurface),
		terminal: &Terminal{},
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

// loadChunks runs the initial streaming pass. It eagerly loads the LOD 0
// square so the player spawns with immediate ground under their feet, then
// hands off to streamChunks to schedule the rest over subsequent frames.
//
// The eager pass is capped to the full LOD 0 square because we only block
// here once, at startup. All later streaming uses the budgeted path in
// streamChunks so movement never stalls the main thread.
func (game *Game) loadChunks() {
	for dx := -lod0Radius; dx <= lod0Radius; dx++ {
		for dz := -lod0Radius; dz <= lod0Radius; dz++ {
			coord := Point2D{game.middle.x + dx, game.middle.y + dz}
			if _, ok := game.chunks[coord]; ok {
				continue
			}
			game.chunks[coord] = game.loadChunk(coord)
		}
	}
	// Let streamChunks populate the distant surface ring and set up any
	// bookkeeping both maps share.
	game.streamChunks()
}

// UnloadChunks flushes all in-memory LOD 0 chunks to disk. Safe to call
// from the main thread at shutdown. Distant surfaces are discarded without
// serialization — they are deterministic and can be recomputed from the
// terrain generator.
func (game *Game) UnloadChunks() {
	game.mu.Lock()
	defer game.mu.Unlock()
	for _, c := range game.chunks {
		game.saveChunk(c)
	}
	game.chunks = make(map[Point2D]*Chunk)
	game.surfaces = make(map[Point2D]*ChunkSurface)
}

// newMiddle is called whenever the player crosses a chunk boundary.
// Recomputes the LOD 0 / surface rings relative to the new origin.
func (game *Game) newMiddle(coord Point2D) {
	log.Println("new middle", coord)
	game.middle = coord
	game.streamChunks()
}

// getChunk returns the LOD 0 chunk containing world chunk (x, z), or nil
// when the chunk is not currently loaded. If `force` is true and the
// chunk isn't loaded, it's fetched from disk (or generated) immediately
// and added to the map — used by world-editing paths that need to mutate
// a neighbouring chunk's block data.
func (game *Game) getChunk(x, z int, force bool) *Chunk {
	coord := Point2D{x, z}
	if c, ok := game.chunks[coord]; ok {
		return c
	}
	if force {
		c := game.loadChunk(coord)
		game.chunks[coord] = c
		return c
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
	if chk != nil {
		return chk.Blocks[b.x][b.y][b.z]
	}
	return game.inferBlockFromSurface(x, y, z)
}

// inferBlockFromSurface returns an approximate block id for a chunk we don't
// have fully loaded but for which we have a distant surface. Used so the
// chunk mesher can correctly cull faces between LOD 0 and distant chunks.
func (game *Game) inferBlockFromSurface(x, y, z int) uint8 {
	if y < 0 || y >= worldHeight {
		return 0
	}
	coord, b := game.getChunkBlockAt(x, y, z)
	surf, ok := game.surfaces[*coord]
	if !ok {
		return 0
	}
	h := surf.Heights[b.x][b.z]
	s := surf.Surface[b.x][b.z]
	switch {
	case y > h:
		return IDAir
	case y == h:
		return s
	default:
		return IDStone
	}
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
