package game

// Distant-tier sector meshing.
//
// A sector is a square of chunks (edge = lodSectorSize(tier) chunks).
// BuildSectorMesh rasterises every column in the sector at a stride of
// `step = 1 << tier` blocks, emitting one top quad per sample plus side
// skirts to lower neighbours (both within-sector and cross-sector).
//
// Vertex data uses the same layout as the regular chunk mesh so draws
// share the chunk shader. Per-vertex light is the ambient-outdoor
// fallback (block light = 0, sky light = 1).
//
// Water handling:
//   - Tier 1/2 (step 2/4) routes water to the translucent stream so it
//     blends correctly with opaque terrain below.
//   - Tier 3/4 (step 8/16) omits water entirely. At those strides the
//     mesh cells are coarser than visible water extent; the clear colour
//     (sky blue) shows through and reads as distant ocean.

// lodSampler returns the visible surface height and block id at world
// column (wx, wz). Implementations either read a cached ChunkSurface
// (LOD 1/2) or sample the terrain generator directly (LOD 3/4).
type lodSampler func(wx, wz int) (h int, id uint8)

// BuildSectorMesh emits the opaque and translucent vertex streams for a
// sector at the given tier. `sectorCoord` is the lower-left chunk of
// the sector. `sample` supplies column data; it must be safe to call for
// any column inside the sector and for one-step neighbours outside
// (needed for cross-sector skirts).
func BuildSectorMesh(sectorCoord Point2D, tier int, sample lodSampler) (opaque, translucent []ChunkVertex) {
	step := stepForLOD(tier)
	if step < 1 {
		step = 1
	}
	size := lodSectorSize(tier)
	sectorBlocks := size * 16

	baseX := sectorCoord.x << 4
	baseZ := sectorCoord.y << 4

	// Coarse sector means fewer cells; allocate a small starting capacity.
	cells := (sectorBlocks / step) * (sectorBlocks / step)
	opaque = make([]ChunkVertex, 0, cells*6)
	translucent = make([]ChunkVertex, 0, cells)

	// Iterate sector-local block coordinates in steps. The sampler is
	// called once per cell plus once per neighbouring cell for skirts;
	// that's ~5 calls per cell. At tier 4 this is ~225 calls per sector
	// total — cheap.
	for bx := 0; bx < sectorBlocks; bx += step {
		for bz := 0; bz < sectorBlocks; bz += step {
			wx := baseX + bx
			wz := baseZ + bz
			h, id := sample(wx, wz)
			if id == IDAir {
				continue
			}

			info := Block(id)
			if info == nil {
				continue
			}
			color := info.Color

			isWater := id == IDWater
			alpha := float32(1.0)
			if isWater {
				alpha = info.Alpha
			}

			x0 := float32(wx)
			x1 := float32(wx + step)
			z0 := float32(wz)
			z1 := float32(wz + step)
			yTop := float32(h + 1)

			if isWater {
				appendSurfaceTop(&translucent, x0, x1, yTop, z0, z1, color, alpha)
			} else {
				appendSurfaceTop(&opaque, x0, x1, yTop, z0, z1, color, alpha)
			}

			// Side skirts — every direction. Skirts go to opaque unless
			// the current cell is water (translucent side for water, but
			// water skirts only form when the neighbour is lower, which
			// means it's land below sea level — which we already flooded
			// to water so the neighbour height is also at sea level.
			// Side skirts for water therefore almost never emit, but we
			// still route them correctly when they do).
			emitSkirt := func(nh int, axis byte, dir int) {
				if nh >= h {
					return
				}
				y0 := float32(nh + 1)
				y1 := yTop
				switch axis {
				case 'x':
					xPlane := x1
					if dir < 0 {
						xPlane = x0
					}
					if isWater {
						appendSurfaceSideX(&translucent, xPlane, y0, y1, z0, z1, dir, color, alpha)
					} else {
						appendSurfaceSideX(&opaque, xPlane, y0, y1, z0, z1, dir, color, alpha)
					}
				case 'z':
					zPlane := z1
					if dir < 0 {
						zPlane = z0
					}
					if isWater {
						appendSurfaceSideZ(&translucent, zPlane, y0, y1, x0, x1, dir, color, alpha)
					} else {
						appendSurfaceSideZ(&opaque, zPlane, y0, y1, x0, x1, dir, color, alpha)
					}
				}
			}

			// Neighbour cells are sampled at their cell ORIGIN (step away),
			// not at wx-1 / wz-1 which reads inside the current cell's
			// neighbour instead of at its grid corner. Using the step
			// matches the cell-aligned sampling pattern.
			{
				nh, _ := sample(wx+step, wz)
				emitSkirt(nh, 'x', +1)
			}
			{
				nh, _ := sample(wx-step, wz)
				emitSkirt(nh, 'x', -1)
			}
			{
				nh, _ := sample(wx, wz+step)
				emitSkirt(nh, 'z', +1)
			}
			{
				nh, _ := sample(wx, wz-step)
				emitSkirt(nh, 'z', -1)
			}
		}
	}
	return opaque, translucent
}

// appendSurfaceTop emits a top (+Y) quad for a distant surface cell.
// Winding matches FaceTop in the main mesher (CCW from above).
func appendSurfaceTop(out *[]ChunkVertex, x0, x1, y, z0, z1 float32, color RGBA, alpha float32) {
	r, g, b := colorFloats(color)
	n := [3]float32{0, 1, 0}
	v := [4][3]float32{
		{x0, y, z0},
		{x0, y, z1},
		{x1, y, z1},
		{x1, y, z0},
	}
	emitTriQuad(out, v, n, r, g, b, alpha)
}

// appendSurfaceSideX emits a skirt quad perpendicular to the X axis at
// world X = x. dir > 0 means the quad faces +X (the lower neighbour is
// on the +X side), dir < 0 means it faces -X.
func appendSurfaceSideX(out *[]ChunkVertex, x, y0, y1, z0, z1 float32, dir int, color RGBA, alpha float32) {
	if y1 <= y0 {
		return
	}
	r, g, b := colorFloats(color)
	var n [3]float32
	var v [4][3]float32
	if dir > 0 {
		n = [3]float32{1, 0, 0}
		v = [4][3]float32{
			{x, y0, z0},
			{x, y1, z0},
			{x, y1, z1},
			{x, y0, z1},
		}
	} else {
		n = [3]float32{-1, 0, 0}
		v = [4][3]float32{
			{x, y0, z0},
			{x, y0, z1},
			{x, y1, z1},
			{x, y1, z0},
		}
	}
	emitTriQuad(out, v, n, r, g, b, alpha)
}

// appendSurfaceSideZ mirrors appendSurfaceSideX for the Z axis.
func appendSurfaceSideZ(out *[]ChunkVertex, z, y0, y1, x0, x1 float32, dir int, color RGBA, alpha float32) {
	if y1 <= y0 {
		return
	}
	r, g, b := colorFloats(color)
	var n [3]float32
	var v [4][3]float32
	if dir > 0 {
		n = [3]float32{0, 0, 1}
		v = [4][3]float32{
			{x0, y0, z},
			{x1, y0, z},
			{x1, y1, z},
			{x0, y1, z},
		}
	} else {
		n = [3]float32{0, 0, -1}
		v = [4][3]float32{
			{x0, y0, z},
			{x0, y1, z},
			{x1, y1, z},
			{x1, y0, z},
		}
	}
	emitTriQuad(out, v, n, r, g, b, alpha)
}

// emitTriQuad appends the two triangles of a CCW-wound quad to `out`.
// Light attributes are fixed to (0, 0, 0, 1) — no block light, full
// sky light — so distant terrain matches the ambient outdoor look.
func emitTriQuad(out *[]ChunkVertex, v [4][3]float32, n [3]float32, r, g, b, a float32) {
	mk := func(p [3]float32) ChunkVertex {
		return ChunkVertex{
			X: p[0], Y: p[1], Z: p[2],
			Nx: n[0], Ny: n[1], Nz: n[2],
			R: r, G: g, B: b, A: a,
			LR: 0, LG: 0, LB: 0, LSky: 1,
		}
	}
	*out = append(*out, mk(v[0]), mk(v[1]), mk(v[2]))
	*out = append(*out, mk(v[0]), mk(v[2]), mk(v[3]))
}

func colorFloats(c RGBA) (r, g, b float32) {
	return float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255
}
