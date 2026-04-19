package game

// Light propagation for chunk voxel lighting.
//
// Two channels are stored per chunk: SkyLight (a single 0..15 scalar from the
// open sky) and BlockLight (packed RGB, each channel 0..15, emitted by
// emissive blocks). Both are recomputed from scratch every time a chunk is
// loaded or a block is edited in that chunk — keeping the implementation
// simple; incremental relighting can come later.
//
// The propagation runs BFS flood-fill within the chunk. Neighbouring chunks
// do their own propagation, so boundary continuity is approximate: lanterns
// near a chunk edge will dim at the boundary instead of bleeding smoothly
// into the neighbour. This is acceptable for v1.

// packLight packs three 0..15 channels into a 12-bit value
// (R<<8 | G<<4 | B).
func packLight(r, g, b uint8) uint16 {
	return uint16(r&15)<<8 | uint16(g&15)<<4 | uint16(b&15)
}

// unpackLight splits a packed light value back into its three channels.
func unpackLight(l uint16) (r, g, b uint8) {
	return uint8((l >> 8) & 15), uint8((l >> 4) & 15), uint8(l & 15)
}

// lightTraversable reports whether light may pass through the block at id.
// Air and translucent blocks (e.g. water, leaves) allow propagation; opaque
// blocks stop it. The per-block metadata is the source of truth.
func lightTraversable(id uint8) bool {
	info := Block(id)
	if info == nil {
		return true
	}
	return info.Transparent || info.Translucent
}

// lightQueueEntry is a BFS queue entry for sky-light propagation.
type lightQueueEntry struct {
	x, y, z int
	level   uint8
}

// PropagateChunkLight recomputes both SkyLight and BlockLight for the given
// chunk from scratch. worldGet is used only to look up block ids — neighbour
// chunks do their own propagation and are not mutated here.
func PropagateChunkLight(chunk *Chunk, worldGet func(x, y, z int) uint8) {
	if chunk == nil {
		return
	}

	// Reset both buffers.
	for bx := 0; bx < 16; bx++ {
		for by := 0; by < worldHeight; by++ {
			for bz := 0; bz < 16; bz++ {
				chunk.SkyLight[bx][by][bz] = 0
				chunk.BlockLight[bx][by][bz] = 0
			}
		}
	}

	propagateSkyLight(chunk)
	propagateBlockLight(chunk)
}

// propagateSkyLight seeds each column with sky level 15 down through
// traversable blocks, then BFS-spreads horizontally within the chunk with a
// decay of 1 per step.
func propagateSkyLight(chunk *Chunk) {
	queue := make([]lightQueueEntry, 0, 1024)

	// Vertical fill: starting from the top, set every traversable voxel to 15
	// until the column hits a blocker.
	for bx := 0; bx < 16; bx++ {
		for bz := 0; bz < 16; bz++ {
			for by := worldHeight - 1; by >= 0; by-- {
				id := chunk.Blocks[bx][by][bz]
				if !lightTraversable(id) {
					break
				}
				chunk.SkyLight[bx][by][bz] = 15
				queue = append(queue, lightQueueEntry{bx, by, bz, 15})
			}
		}
	}

	// BFS: spread sideways with a -1 decay per step.
	for head := 0; head < len(queue); head++ {
		e := queue[head]
		if e.level == 0 {
			continue
		}
		next := e.level - 1

		for _, off := range faceOffsets {
			nx := e.x + off.x
			ny := e.y + off.y
			nz := e.z + off.z
			if nx < 0 || nx > 15 || nz < 0 || nz > 15 || ny < 0 || ny >= worldHeight {
				continue
			}
			if !lightTraversable(chunk.Blocks[nx][ny][nz]) {
				continue
			}
			if chunk.SkyLight[nx][ny][nz] >= next {
				continue
			}
			chunk.SkyLight[nx][ny][nz] = next
			queue = append(queue, lightQueueEntry{nx, ny, nz, next})
		}
	}
}

// blockLightQueueEntry holds the per-channel BFS state for coloured light.
type blockLightQueueEntry struct {
	x, y, z int
	r, g, b uint8
}

// propagateBlockLight seeds emissive blocks then BFS-propagates each RGB
// channel independently (taking the max over all paths), decaying by 1 per
// step. Only traversable cells receive light; opaque blocks neither store nor
// forward coloured light.
func propagateBlockLight(chunk *Chunk) {
	queue := make([]blockLightQueueEntry, 0, 256)

	// Seed pass.
	for bx := 0; bx < 16; bx++ {
		for by := 0; by < worldHeight; by++ {
			for bz := 0; bz < 16; bz++ {
				id := chunk.Blocks[bx][by][bz]
				info := Block(id)
				if info == nil || !info.Emissive {
					continue
				}
				packed := packLight(info.LightR, info.LightG, info.LightB)
				chunk.BlockLight[bx][by][bz] = packed
				queue = append(queue, blockLightQueueEntry{
					x: bx, y: by, z: bz,
					r: info.LightR, g: info.LightG, b: info.LightB,
				})
			}
		}
	}

	// BFS: propagate each channel independently.
	for head := 0; head < len(queue); head++ {
		e := queue[head]
		if e.r == 0 && e.g == 0 && e.b == 0 {
			continue
		}
		nr := uint8(0)
		ng := uint8(0)
		nb := uint8(0)
		if e.r > 0 {
			nr = e.r - 1
		}
		if e.g > 0 {
			ng = e.g - 1
		}
		if e.b > 0 {
			nb = e.b - 1
		}

		for _, off := range faceOffsets {
			nx := e.x + off.x
			ny := e.y + off.y
			nz := e.z + off.z
			if nx < 0 || nx > 15 || nz < 0 || nz > 15 || ny < 0 || ny >= worldHeight {
				continue
			}
			if !lightTraversable(chunk.Blocks[nx][ny][nz]) {
				continue
			}
			cur := chunk.BlockLight[nx][ny][nz]
			cr, cg, cb := unpackLight(cur)
			changed := false
			if nr > cr {
				cr = nr
				changed = true
			}
			if ng > cg {
				cg = ng
				changed = true
			}
			if nb > cb {
				cb = nb
				changed = true
			}
			if !changed {
				continue
			}
			chunk.BlockLight[nx][ny][nz] = packLight(cr, cg, cb)
			queue = append(queue, blockLightQueueEntry{
				x: nx, y: ny, z: nz,
				r: cr, g: cg, b: cb,
			})
		}
	}
}

// propagateChunkLight is a thin wrapper bound to the game so call-sites read
// naturally. It uses getBlockAt to look up block ids in neighbour chunks; the
// current BFS is chunk-local but leaves the hook in place for future work.
func (game *Game) propagateChunkLight(chunk *Chunk) {
	if chunk == nil {
		return
	}
	PropagateChunkLight(chunk, game.getBlockAt)
}

// sampleLight returns the normalized (0..1) block-light RGB and sky-light for
// the world cell at (nx, ny, nz). Returns the ambient fallback (full sky,
// no block light) for out-of-world queries or chunks that aren't loaded so
// distant/unloaded chunks don't appear as black voids.
func (game *Game) sampleLight(nx, ny, nz int) (r, g, b, sky float32) {
	if ny < 0 || ny >= worldHeight {
		return 0, 0, 0, 1
	}
	chunk, bl := game.getChunkBlockAt(nx, ny, nz)
	chk := game.getChunk(chunk.x, chunk.y, false)
	if chk == nil {
		return 0, 0, 0, 1
	}
	br, bg, bb := unpackLight(chk.BlockLight[bl.x][bl.y][bl.z])
	s := chk.SkyLight[bl.x][bl.y][bl.z]
	const inv = 1.0 / 15.0
	return float32(br) * inv, float32(bg) * inv, float32(bb) * inv, float32(s) * inv
}
