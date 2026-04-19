package game

// LOD (Level of Detail) streaming for extended view distance.
//
// The world is split into three conceptual zones centred on the player's
// current chunk (`Game.middle`):
//
//   - LOD 0 (<= lod0Radius chunks away): fully loaded chunk with block, sky
//     and block-light data. Physics and lighting run here. Meshing uses the
//     per-block greedy mesher.
//   - LOD far (> lod0Radius, <= lodMacroRadius): only a ChunkSurface (a
//     16x16 heightmap + surface block id per column) is kept. The distant
//     mesh is built by BuildSurfaceMesh. No gob file is read — the terrain
//     generator is deterministic so `heightAt` + `biomeAt` reproduce the
//     surface cheaply.
//
// The intermediate LOD 1/2/3 tiers described in the design brief collapse
// to a single "far" tier in v1: one sample per column (step = 1), with side
// faces emitted when a neighbour column is lower so the horizon keeps a
// blocky 3D profile.
const (
	// lod0Radius is the half-width (in chunks) of the fully loaded LOD 0
	// square centred on the player. radius 4 → 9x9 = 81 active chunks.
	lod0Radius = 4
	// lodFarRadius marks the inner boundary beyond which only surface
	// meshes are built. (Currently equal to lod0Radius because the LOD 1
	// tier is not implemented separately in v1.)
	lodFarRadius = 12
	// lodMacroRadius is the horizon: chunks farther than this are fully
	// unloaded (no block data, no surface mesh).
	lodMacroRadius = 32

	// distantLODStep is the sampling stride used by BuildSurfaceMesh for
	// the distant tier. 1 = per-column detail; bump to 2 or 4 for cheaper
	// meshes at the cost of visible stair-stepping. Kept at 1 so the first
	// LOD ring beyond the active square remains readable.
	distantLODStep = 1
)

// ChunkSurface is a cheap, physics-less summary of a chunk used by the
// distant LOD mesher. It's derived entirely from the deterministic terrain
// generator (no gob load, no block grid) so populating it is just noise
// queries.
type ChunkSurface struct {
	Coordinates Point2D
	// Heights[bx][bz] is the Y coordinate of the topmost *solid* surface
	// block for that column (i.e. the surface we render, which is water
	// when terrain dips below sea level).
	Heights [16][16]int
	// Surface[bx][bz] is the block id rendered at Heights[bx][bz].
	Surface [16][16]uint8
}

// computeSurface derives the 16x16 heightmap and surface block id for the
// chunk at `coord`. This is a pure function of the terrain generator's
// noise; it does not touch disk or allocate the full 16x128x16 block grid.
func (g *terrainGen) computeSurface(coord Point2D) *ChunkSurface {
	surf := &ChunkSurface{Coordinates: coord}

	baseX := coord.x << 4
	baseZ := coord.y << 4

	for bx := 0; bx < 16; bx++ {
		for bz := 0; bz < 16; bz++ {
			wx := baseX + bx
			wz := baseZ + bz
			h := g.heightAt(wx, wz)
			biome := g.biomeAt(wx, wz)
			surface, _, _ := columnBlocks(biome)

			// Mountain peaks are capped with snow (mirrors generateChunk).
			if biome == BiomeMountain && h > 80 {
				surface = IDSnow
			}

			// Water fills dips below sea level; the visible top is then the
			// water surface at sea level.
			if h < seaLevel {
				surf.Heights[bx][bz] = seaLevel
				surf.Surface[bx][bz] = IDWater
			} else {
				surf.Heights[bx][bz] = h
				surf.Surface[bx][bz] = surface
			}
		}
	}
	return surf
}

// chunkDistSq returns the squared Chebyshev-ish distance between (cx, cz)
// and `middle`. Using squared Euclidean chunk distance gives a natural
// radial priority order for streaming.
func chunkDistSq(middle Point2D, cx, cz int) int {
	dx := cx - middle.x
	dz := cz - middle.y
	return dx*dx + dz*dz
}
