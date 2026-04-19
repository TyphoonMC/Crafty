package game

// LOD (Level of Detail) streaming for extended view distance.
//
// The world is split into a cascade of concentric rings centred on the
// player's current chunk (`Game.middle`). The outermost ring sets the
// horizon at 240 chunks * 16 blocks = 3840 blocks.
//
//   - LOD 0 (Chebyshev distance <= lodRadii[0]): fully loaded chunk with
//     block, sky and block-light data. Physics and lighting run here.
//     Meshing uses the per-block greedy mesher. Always streamed — never
//     frustum-culled — so the player never falls through the floor.
//   - LOD 1..N: per-sector surface meshes built from a 16x16 heightmap
//     summary (LOD 1/2) or on-the-fly samples from the deterministic
//     terrain generator (LOD 3/4). Sector size scales with the tier to
//     keep draw-call counts bounded.
//
// Ring i (1-indexed) covers chunk distances in the half-open interval
// (lodRadii[i-1], lodRadii[i]]. Tier 0 is the full-detail square.
//
// Sector sizing follows `lodSectorSize = 1 << tier`:
//
//     tier 1: 2x2  chunks per sector   (~250 sectors)
//     tier 2: 4x4  chunks per sector   (~560 sectors)
//     tier 3: 8x8  chunks per sector   (~200 sectors)
//     tier 4: 16x16 chunks per sector  (~225 sectors)
//
// Each sector gets one `lodMesh` that batches every chunk it covers into
// a single draw call. Total ~1250 LOD draw calls + ~81 LOD 0 chunks.
const (
	// numLODTiers counts the distant tiers (LOD 1..4). Tier 0 is handled
	// separately by the regular chunk pipeline.
	numLODTiers = 4
	// maxLODTier is the highest tier index (inclusive).
	maxLODTier = numLODTiers // 4, since tier 0 = full, tiers 1..4 = cascade
	// cullMargin extends the frustum by this many chunks on every side
	// when deciding whether a sector is relevant. Gives rotation a bit of
	// hysteresis so turning the camera doesn't immediately re-mesh.
	cullMargin = 1
	// lruMeshBudget is the maximum number of LOD sector meshes kept in
	// memory. Sectors beyond the horizon or outside the frustum fall out
	// of this LRU in least-recently-used order so head-turning doesn't
	// re-generate meshes every frame.
	lruMeshBudget = 4000
	// streamMeshBudget caps sector meshes built per streaming tick. A
	// sector covers many chunks so even a small budget fills the horizon
	// in a few seconds.
	streamMeshBudget = 8
	// streamLOD0Budget caps per-frame LOD 0 chunk loads (disk read +
	// terrain gen + lighting).
	streamLOD0Budget = 4
	// streamSurfaceBudget caps per-frame cached surface builds
	// (LOD 1/2). Lower than before because computeSurface now runs the
	// full generateChunk pass so trees appear at distance — costs ~1 ms
	// per chunk vs ~0.05 ms when it was noise-only.
	streamSurfaceBudget = 8
)

// lodRadii holds the outer chunk-distance of each LOD tier. Indices line
// up with tier numbers: lodRadii[0] is LOD 0's radius, lodRadii[4] is
// LOD 4's horizon. Ring i covers radii in (lodRadii[i-1], lodRadii[i]].
var lodRadii = [...]int{4, 16, 48, 112, 240}

// lod0Radius is a legacy alias kept for the many call sites that mean
// "the full-detail square half-width".
const lod0Radius = 4

// stepForLOD returns the sampling stride in blocks for the given LOD
// tier. Tier 0 uses per-block meshing so the value is 1; distant tiers
// double the step each level.
func stepForLOD(tier int) int {
	if tier <= 0 {
		return 1
	}
	return 1 << tier
}

// lodSectorSize returns the edge length in chunks of a sector at the
// given tier. Tier 0 has no sectors (chunks render individually) so the
// function returns 1 there too.
func lodSectorSize(tier int) int {
	if tier <= 0 {
		return 1
	}
	return 1 << tier
}

// lodForChunk returns the LOD tier of chunk `coord` relative to
// `middle`, or -1 when the chunk is beyond the outermost ring.
func lodForChunk(middle, coord Point2D) int {
	dx := coord.x - middle.x
	if dx < 0 {
		dx = -dx
	}
	dz := coord.y - middle.y
	if dz < 0 {
		dz = -dz
	}
	r := dx
	if dz > r {
		r = dz
	}
	for i, outer := range lodRadii {
		if r <= outer {
			return i
		}
	}
	return -1
}

// sectorForChunk returns the sector coordinate (lower-left chunk of the
// sector) covering `coord` at the given tier. For tier 0 this is just
// the chunk itself. Sectors snap to a world-space grid of size
// `lodSectorSize(tier)` so the same sector coord is stable no matter
// which chunk in it triggered a recompute.
func sectorForChunk(coord Point2D, tier int) Point2D {
	size := lodSectorSize(tier)
	// Floor-divide toward -infinity so sectors align consistently across
	// the negative half-space.
	sx := coord.x
	sz := coord.y
	if sx < 0 {
		sx -= size - 1
	}
	if sz < 0 {
		sz -= size - 1
	}
	sx = (sx / size) * size
	sz = (sz / size) * size
	return Point2D{sx, sz}
}

// sectorAABB returns the world-space axis-aligned bounding box of the
// sector starting at `sectorCoord` at the given tier. Y spans the full
// world height so the test works before heights are known.
func sectorAABB(sectorCoord Point2D, tier int) (minX, minY, minZ, maxX, maxY, maxZ float32) {
	size := lodSectorSize(tier)
	minX = float32(sectorCoord.x << 4)
	minZ = float32(sectorCoord.y << 4)
	maxX = float32((sectorCoord.x + size) << 4)
	maxZ = float32((sectorCoord.y + size) << 4)
	minY = 0
	maxY = float32(worldHeight)
	return
}

// ChunkSurface is a cheap, physics-less summary of a chunk used by the
// distant LOD mesher (tiers 1 and 2 cache them). Tier 3/4 skip the cache
// and sample the generator directly — at step 8/16 they only need 4 or 1
// samples per chunk, so caching would waste memory.
//
// Size: 16*16*(8+1) = 2304 bytes per chunk. At tier 2's 16*16 = 256
// chunks-per-sector radius (~5400 chunks in tiers 1+2 combined) this
// stays under ~12 MB total even at the furthest streaming horizon.
type ChunkSurface struct {
	Coordinates Point2D
	// Heights[bx][bz] is the Y coordinate of the topmost *solid* surface
	// block for that column (i.e. the surface we render, which is water
	// when terrain dips below sea level).
	Heights [16][16]int
	// Surface[bx][bz] is the block id rendered at Heights[bx][bz].
	Surface [16][16]uint8
}

// computeSurface runs the full terrain generator (including tree
// placement) into a temporary Chunk and extracts the topmost non-air
// block per column. Trees therefore contribute leaves/wood to the LOD
// surface so distant forests render as visible canopies instead of bare
// biome-colour terrain. Used for the cached tiers (LOD 1/2).
//
// Cost: one generateChunk pass (~32k block-fill ops + Perlin queries).
// The result is cached so the cost is paid once per chunk per streaming
// session. Budget `streamSurfaceBudget` controls the per-frame cap.
func (g *terrainGen) computeSurface(coord Point2D) *ChunkSurface {
	var c Chunk
	c.Coordinates = coord
	g.generateChunk(&c)
	return extractSurface(&c)
}

// extractSurface scans every column of c for the topmost non-air block
// and records its Y + block id on a new ChunkSurface. Shared helper so
// any caller holding a fully generated Chunk derives its distant-tier
// summary the same way.
func extractSurface(c *Chunk) *ChunkSurface {
	surf := &ChunkSurface{Coordinates: c.Coordinates}
	for bx := 0; bx < 16; bx++ {
		for bz := 0; bz < 16; bz++ {
			surf.Heights[bx][bz] = 0
			surf.Surface[bx][bz] = IDAir
			for y := worldHeight - 1; y >= 0; y-- {
				id := c.Blocks[bx][y][bz]
				if id == IDAir {
					continue
				}
				surf.Heights[bx][bz] = y
				surf.Surface[bx][bz] = id
				break
			}
		}
	}
	return surf
}

// sampleSurface returns (height, surface block id) at world column
// (wx, wz). Factored out so both ChunkSurface construction (LOD 1/2)
// and on-the-fly sampling (LOD 3/4) share the same biome/water rules.
func (g *terrainGen) sampleSurface(wx, wz int) (int, uint8) {
	h := g.heightAt(wx, wz)
	biome := g.biomeAt(wx, wz)
	surface, _, _ := columnBlocks(biome)

	// Mountain peaks cap with snow (mirrors generateChunk).
	if biome == BiomeMountain && h > 80 {
		surface = IDSnow
	}

	if h < seaLevel {
		return seaLevel, IDWater
	}
	return h, surface
}

// chunkDistSq returns squared Euclidean chunk distance. Used as a
// streaming priority so near work wins over far work each tick.
func chunkDistSq(middle Point2D, cx, cz int) int {
	dx := cx - middle.x
	dz := cz - middle.y
	return dx*dx + dz*dz
}
