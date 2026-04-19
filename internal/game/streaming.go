package game

import "sort"

// Streaming tick: keep the LOD 0 square, the cached surface tiers
// (LOD 1/2), and the sector-mesh cache (LOD 1..4) in sync with the
// player's current chunk. Frustum culling applies to every tier except
// LOD 0 — the chunk the player stands on always streams so physics
// can't fall through the floor regardless of where the camera faces.
//
// Per-frame budgets:
//   - `streamLOD0Budget`    LOD 0 chunks (disk load / terrain gen + lighting).
//   - `streamSurfaceBudget` cached surfaces (LOD 1/2 only, cheap noise).
//   - `streamMeshBudget`    sector meshes uploaded to the GPU.
//
// Mesh work is not done here — streamChunks only primes surface caches
// and picks the sectors that should be meshed. The actual
// BuildSectorMesh + GPU upload runs in `refreshLODMeshes` on the GL
// thread so we don't touch OpenGL from arbitrary callers.

// streamChunks updates the in-memory chunk maps and the sector-mesh
// wishlist around `game.middle`. Must run with `game.mu` held
// (callers: NewGame, newMiddle, drawScene).
func (game *Game) streamChunks() {
	game.streamLOD0()
	game.streamCachedSurfaces()
	game.streamLODSectors()
}

// streamLOD0 ensures the full-detail square around the player is
// loaded. The player's own chunk always loads even when the budget is
// exhausted so physics runs on the very first frame after a teleport.
func (game *Game) streamLOD0() {
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
	sort.Slice(missing, func(i, j int) bool {
		return chunkDistSq(game.middle, missing[i].x, missing[i].y) <
			chunkDistSq(game.middle, missing[j].x, missing[j].y)
	})
	// Always-on: the chunk under the player.
	for _, coord := range missing {
		if coord == game.middle {
			game.chunks[coord] = game.loadChunk(coord)
			break
		}
	}
	budget := streamLOD0Budget
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

	// Evict LOD 0 chunks that fell outside the square.
	for coord, c := range game.chunks {
		if _, ok := active[coord]; ok {
			continue
		}
		game.saveChunk(c)
		delete(game.chunks, coord)
	}
}

// streamCachedSurfaces populates `game.surfaces` for every chunk that
// falls inside the cached tiers (LOD 1 and LOD 2). Frustum-culled —
// chunks outside the expanded view frustum never have their surface
// computed so we stay well below the naive 175 MB all-chunks footprint.
func (game *Game) streamCachedSurfaces() {
	active := make(map[Point2D]struct{})
	missing := make([]Point2D, 0)

	outer := lodRadii[2] // last cached tier is LOD 2
	for dx := -outer; dx <= outer; dx++ {
		for dz := -outer; dz <= outer; dz++ {
			coord := Point2D{game.middle.x + dx, game.middle.y + dz}
			tier := lodForChunk(game.middle, coord)
			if tier <= 0 || tier > 2 {
				continue
			}
			if !game.frustum.ChunkVisible(coord) {
				continue
			}
			active[coord] = struct{}{}
			if _, ok := game.surfaces[coord]; !ok {
				missing = append(missing, coord)
			}
		}
	}

	sort.Slice(missing, func(i, j int) bool {
		return chunkDistSq(game.middle, missing[i].x, missing[i].y) <
			chunkDistSq(game.middle, missing[j].x, missing[j].y)
	})
	budget := streamSurfaceBudget
	for _, coord := range missing {
		if budget <= 0 {
			break
		}
		game.surfaces[coord] = game.gen.computeSurface(coord)
		budget--
	}

	// Evict cached surfaces that are no longer in any cached tier or
	// fell out of the frustum. Keep a small hysteresis by retaining
	// surfaces for one frame past visibility loss — simplest form is to
	// just drop them immediately and let the cheap noise recompute on
	// re-entry. Noise evals are ~1 us/column, << frame budget.
	for coord := range game.surfaces {
		tier := lodForChunk(game.middle, coord)
		keep := tier >= 1 && tier <= 2 && game.frustum.ChunkVisible(coord)
		if !keep {
			delete(game.surfaces, coord)
		}
	}
}

// pendingSector is a queue entry for sector meshes that need to be
// (re)built this frame.
type pendingSector struct {
	key  lodMeshKey
	dist int
}

// streamLODSectors fills the `game.pendingSectors` queue with sector
// keys that should be meshed on the GL thread during refreshLODMeshes.
// The queue is size-bounded here so we don't accumulate stale work.
func (game *Game) streamLODSectors() {
	game.pendingSectors = game.pendingSectors[:0]
	game.frustumSectors = game.frustumSectors[:0]
	seen := game.sectorSetScratch
	for k := range seen {
		delete(seen, k)
	}
	if seen == nil {
		seen = make(map[lodMeshKey]struct{})
		game.sectorSetScratch = seen
	}

	horizon := lodRadii[maxLODTier]
	mid := game.middle

	// Walk every sector covering every tier ring. The sector grid is
	// aligned to multiples of `lodSectorSize(tier)`, so we snap the
	// iteration bounds outward before stepping.
	for tier := 1; tier <= maxLODTier; tier++ {
		size := lodSectorSize(tier)
		outer := lodRadii[tier]
		inner := lodRadii[tier-1]

		minSX := snapDown(mid.x-outer, size)
		maxSX := snapDown(mid.x+outer, size)
		minSZ := snapDown(mid.y-outer, size)
		maxSZ := snapDown(mid.y+outer, size)

		for sx := minSX; sx <= maxSX; sx += size {
			for sz := minSZ; sz <= maxSZ; sz += size {
				sector := Point2D{sx, sz}
				if !sectorTouchesTier(sector, size, mid, inner, outer) {
					continue
				}
				if !game.frustum.SectorVisible(sector, tier) {
					continue
				}
				key := lodMeshKey{tier: tier, sector: sector}
				seen[key] = struct{}{}
				game.frustumSectors = append(game.frustumSectors, key)

				if game.renderer != nil {
					if _, ok := game.renderer.lodMeshes[key]; ok {
						continue
					}
				}
				cx := sector.x + size/2
				cz := sector.y + size/2
				d := chunkDistSq(mid, cx, cz)
				game.pendingSectors = append(game.pendingSectors, pendingSector{key: key, dist: d})
			}
		}
	}

	// Nearest sectors mesh first so the perceived horizon fills out
	// naturally instead of blotching in far patches before near ones.
	sort.Slice(game.pendingSectors, func(i, j int) bool {
		return game.pendingSectors[i].dist < game.pendingSectors[j].dist
	})

	// Belt-and-braces: drop sector meshes whose key is far beyond the
	// horizon. LRU handles the in-budget case; this is just the safety
	// net for large teleports.
	if game.renderer != nil {
		for k := range game.renderer.lodMeshes {
			if k.tier < 1 || k.tier > maxLODTier {
				continue
			}
			dx := k.sector.x - mid.x
			if dx < 0 {
				dx = -dx
			}
			dz := k.sector.y - mid.y
			if dz < 0 {
				dz = -dz
			}
			chebDist := dx
			if dz > chebDist {
				chebDist = dz
			}
			if chebDist > horizon+lodSectorSize(k.tier) {
				if m, ok := game.renderer.lodMeshes[k]; ok {
					game.renderer.freeLODMesh(m)
					delete(game.renderer.lodMeshes, k)
				}
			}
		}
	}
}

// snapDown floors `v` down to the nearest multiple of `size`, handling
// the negative half-space correctly.
func snapDown(v, size int) int {
	if v >= 0 {
		return (v / size) * size
	}
	return -((-v + size - 1) / size) * size
}

// sectorTouchesTier reports whether any chunk inside the given sector
// lies in the Chebyshev-distance half-open interval (inner, outer]
// relative to `middle`. Used to skip sectors that sit entirely inside
// a nearer tier's ring or entirely outside the horizon.
func sectorTouchesTier(sector Point2D, size int, middle Point2D, inner, outer int) bool {
	// Closest corner of the sector to `middle`.
	sMaxX := sector.x + size - 1
	sMaxZ := sector.y + size - 1
	dxMin := clampAwayFrom(middle.x, sector.x, sMaxX)
	dzMin := clampAwayFrom(middle.y, sector.y, sMaxZ)
	dMin := absInt(dxMin)
	if a := absInt(dzMin); a > dMin {
		dMin = a
	}
	if dMin > outer {
		return false
	}
	// Farthest corner in Chebyshev terms (whichever of sector.x or sMaxX
	// is further from middle.x, same for z).
	dxMax := furthest(middle.x, sector.x, sMaxX)
	dzMax := furthest(middle.y, sector.y, sMaxZ)
	dMax := absInt(dxMax)
	if a := absInt(dzMax); a > dMax {
		dMax = a
	}
	if dMax <= inner {
		return false
	}
	return true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// clampAwayFrom returns 0 when `v` lies in [lo, hi], otherwise the
// signed distance to the nearer endpoint. In other words, the smallest
// |distance| from v to the interval — with the sign of (v - endpoint).
func clampAwayFrom(v, lo, hi int) int {
	if v < lo {
		return v - lo
	}
	if v > hi {
		return v - hi
	}
	return 0
}

// furthest returns the signed distance from v to whichever endpoint of
// [lo, hi] is furthest away.
func furthest(v, lo, hi int) int {
	d1 := v - lo
	d2 := v - hi
	if absInt(d1) >= absInt(d2) {
		return d1
	}
	return d2
}

// refreshLODMeshes runs on the GL thread. It walks the pending-sector
// queue in priority order, builds & uploads meshes up to the per-frame
// budget, and trims the LRU cache back to budget.
func (game *Game) refreshLODMeshes() {
	if game.renderer == nil {
		return
	}
	r := game.renderer
	r.lodClock++
	now := r.lodClock

	// Mark currently-visible sectors fresh so the LRU keeps them.
	for _, key := range game.frustumSectors {
		if m, ok := r.lodMeshes[key]; ok {
			m.lastUsed = now
		}
	}

	budget := streamMeshBudget
	for _, p := range game.pendingSectors {
		if budget <= 0 {
			break
		}
		key := p.key
		if _, ok := r.lodMeshes[key]; ok {
			continue
		}
		opaque, translucent := game.buildSectorMesh(key.sector, key.tier)
		if len(opaque) == 0 && len(translucent) == 0 {
			// Empty sector (all air). Still cache a mesh entry so we
			// don't repeatedly try to build it.
			m := &lodMesh{tier: key.tier, sectorCoord: key.sector, lastUsed: now}
			r.lodMeshes[key] = m
			budget--
			continue
		}
		m := &lodMesh{tier: key.tier, sectorCoord: key.sector, lastUsed: now}
		r.uploadLODMesh(m, opaque, translucent)
		r.lodMeshes[key] = m
		budget--
	}

	// Protect visible sectors from LRU eviction so head-turning doesn't
	// drop meshes we're about to draw.
	pinned := make(map[lodMeshKey]struct{}, len(game.frustumSectors))
	for _, k := range game.frustumSectors {
		pinned[k] = struct{}{}
	}
	r.trimLODLRU(pinned)
}

// buildSectorMesh picks the right sampler for the tier and calls the
// shared builder.
func (game *Game) buildSectorMesh(sector Point2D, tier int) (opaque, translucent []ChunkVertex) {
	sampler := game.sectorSampler(tier)
	return BuildSectorMesh(sector, tier, sampler)
}

// sectorSampler returns a column sampler for the given tier. Tier 1/2
// read from the cached ChunkSurface map (falling back to the generator
// for cross-sector neighbours that were never cached). Tier 3/4 skip
// the cache entirely and sample the generator directly — at stride 8
// or 16 the cache would waste memory.
func (game *Game) sectorSampler(tier int) lodSampler {
	if tier <= 2 {
		gen := game.gen
		surfaces := game.surfaces
		return func(wx, wz int) (int, uint8) {
			cx := wx >> 4
			cz := wz >> 4
			bx := wx & 15
			bz := wz & 15
			if surf, ok := surfaces[Point2D{cx, cz}]; ok {
				return surf.Heights[bx][bz], surf.Surface[bx][bz]
			}
			return gen.sampleSurface(wx, wz)
		}
	}
	gen := game.gen
	return func(wx, wz int) (int, uint8) {
		return gen.sampleSurface(wx, wz)
	}
}
