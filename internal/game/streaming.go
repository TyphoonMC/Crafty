package game

import "sort"

// Streaming tick: keep the LOD 0 square and the distant LOD ring in sync
// with the player's current chunk.
//
// Budgets per frame:
//   - `streamLOD0Budget` LOD 0 chunks (disk load / terrain gen + lighting).
//   - `streamSurfaceBudget` distant surfaces (pure noise, cheap). The
//     actual GPU upload runs later, driven by `refreshChunkMeshes`.
//
// A teleport that dumps the whole streaming set at once therefore does not
// stall the main thread: missing chunks are filled in over the next ~N
// frames in ascending distance order.
const (
	streamLOD0Budget    = 4
	streamSurfaceBudget = 32
)

// streamChunks updates the in-memory chunk maps around `game.middle`. It
// loads missing LOD 0 chunks, computes missing distant surfaces, and
// evicts anything beyond lodMacroRadius. Must run with `game.mu` held
// (callers: NewGame, newMiddle).
func (game *Game) streamChunks() {
	// --- LOD 0 ring ---
	active := make(map[Point2D]struct{}, (2*lod0Radius+1)*(2*lod0Radius+1))
	missing := make([]Point2D, 0)
	for dx := -lod0Radius; dx <= lod0Radius; dx++ {
		for dz := -lod0Radius; dz <= lod0Radius; dz++ {
			coord := Point2D{game.middle.x + dx, game.middle.y + dz}
			active[coord] = struct{}{}
			if _, ok := game.chunks[coord]; !ok {
				missing = append(missing, coord)
			}
		}
	}
	// Cheapest chunks first: nearest to the player.
	sort.Slice(missing, func(i, j int) bool {
		return chunkDistSq(game.middle, missing[i].x, missing[i].y) <
			chunkDistSq(game.middle, missing[j].x, missing[j].y)
	})
	budget := streamLOD0Budget
	// The chunk the player is actually standing on must exist so physics
	// doesn't fall through the floor. Always load it, even when the
	// per-frame budget is exhausted.
	for _, coord := range missing {
		if coord == game.middle {
			game.chunks[coord] = game.loadChunk(coord)
			break
		}
	}
	for _, coord := range missing {
		if budget <= 0 {
			break
		}
		if _, ok := game.chunks[coord]; ok {
			continue
		}
		game.chunks[coord] = game.loadChunk(coord)
		budget--
	}

	// Evict LOD 0 chunks that fell outside the square (save to disk first).
	for coord, c := range game.chunks {
		if _, ok := active[coord]; ok {
			continue
		}
		game.saveChunk(c)
		delete(game.chunks, coord)
	}

	// --- Distant surface ring ---
	surfaceActive := make(map[Point2D]struct{}, (2*lodMacroRadius+1)*(2*lodMacroRadius+1))
	surfaceMissing := make([]Point2D, 0)
	for dx := -lodMacroRadius; dx <= lodMacroRadius; dx++ {
		for dz := -lodMacroRadius; dz <= lodMacroRadius; dz++ {
			coord := Point2D{game.middle.x + dx, game.middle.y + dz}
			// LOD 0 chunks already cover these cells with their real mesh.
			if _, ok := active[coord]; ok {
				continue
			}
			surfaceActive[coord] = struct{}{}
			if _, ok := game.surfaces[coord]; !ok {
				surfaceMissing = append(surfaceMissing, coord)
			}
		}
	}
	sort.Slice(surfaceMissing, func(i, j int) bool {
		return chunkDistSq(game.middle, surfaceMissing[i].x, surfaceMissing[i].y) <
			chunkDistSq(game.middle, surfaceMissing[j].x, surfaceMissing[j].y)
	})
	sbudget := streamSurfaceBudget
	for _, coord := range surfaceMissing {
		if sbudget <= 0 {
			break
		}
		game.surfaces[coord] = game.gen.computeSurface(coord)
		sbudget--
	}

	// Evict distant surfaces that fell outside the horizon ring, and also
	// drop surfaces that have been promoted to LOD 0.
	for coord := range game.surfaces {
		if _, ok := surfaceActive[coord]; ok {
			continue
		}
		delete(game.surfaces, coord)
		// The renderer will clean up the matching LOD mesh on its next
		// pass via evictLODMeshes below.
	}

	// Poke the renderer to drop LOD meshes that no longer have a surface
	// (either promoted to LOD 0 or fully out of range).
	if game.renderer != nil {
		game.renderer.evictLODMeshes(surfaceActive)
	}
}

// refreshLODMeshes (re)uploads the LOD mesh for every chunk surface that
// doesn't already have one. Must run on the GL thread.
func (game *Game) refreshLODMeshes() {
	if game.renderer == nil {
		return
	}
	r := game.renderer
	for coord, surf := range game.surfaces {
		if _, ok := r.lodMeshes[coord]; ok {
			continue
		}
		coord := coord
		opaque, translucent := BuildSurfaceMesh(surf, distantLODStep, func(dx, dz int) *ChunkSurface {
			nb := Point2D{coord.x + dx, coord.y + dz}
			if s, ok := game.surfaces[nb]; ok {
				return s
			}
			// Fall back to LOD 0 chunk heights if the neighbour is a full
			// chunk — computeSurface is pure noise so it's cheap.
			if _, ok := game.chunks[nb]; ok {
				return game.gen.computeSurface(nb)
			}
			return nil
		})
		m := &lodMesh{}
		r.uploadLODMesh(m, opaque, translucent)
		r.lodMeshes[coord] = m
	}
}
