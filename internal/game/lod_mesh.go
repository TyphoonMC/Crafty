package game

// BuildSurfaceMesh produces a simple top-face + side-skirt mesh for the given
// chunk surface at the requested LOD level. `neighbour(dx, dz)` returns the
// neighbouring surface at chunk offset (dx, dz), or nil if unavailable.
// Pass nil to keep the legacy no-seam-fix behaviour (intra-chunk only).
//
// `lod` controls the sampling stride: step = 1 << lod, so LOD 0 = per-block,
// LOD 2 = every 4 blocks. Only the top face (plus side skirts when a
// neighbour column is lower) is emitted — no bottom, no back-side faces
// between adjacent columns.
//
// The mesh is split into two vertex streams: `opaque` for solid terrain
// (grass, stone, snow…) and `translucent` for water tops. Per-vertex light
// is the ambient-outdoor fallback: no block light, full sky light, so the
// shared chunk shader lights the distant terrain correctly without needing
// a second program.
//
// `neighbour(dx, dz)` resolves a neighbouring chunk surface so skirts can
// be emitted across chunk boundaries, eliminating visible gaps at the
// distant-tier seams.
func BuildSurfaceMesh(surf *ChunkSurface, lod int, neighbour func(dx, dz int) *ChunkSurface) (opaque, translucent []ChunkVertex) {
	if surf == nil {
		return nil, nil
	}
	step := 1 << lod
	if step < 1 {
		step = 1
	}
	if step > 16 {
		step = 16
	}

	baseX := surf.Coordinates.x << 4
	baseZ := surf.Coordinates.y << 4

	opaque = make([]ChunkVertex, 0, 64)
	translucent = make([]ChunkVertex, 0, 16)

	// neighbourHeight fetches the height of the column at local coordinates
	// (nbx, nbz) inside the neighbour chunk at offset (dx, dz). Returns
	// (0, false) when the neighbour is unavailable (nil closure or nil
	// surface), causing the caller to skip the cross-chunk skirt.
	neighbourHeight := func(dx, dz, nbx, nbz int) (int, bool) {
		if neighbour == nil {
			return 0, false
		}
		ns := neighbour(dx, dz)
		if ns == nil {
			return 0, false
		}
		return ns.Heights[nbx][nbz], true
	}

	// Top quads: one per sample cell. We pick the representative column at
	// (bx, bz) and extend the quad by `step` along +X and +Z.
	for bx := 0; bx < 16; bx += step {
		for bz := 0; bz < 16; bz += step {
			id := surf.Surface[bx][bz]
			if id == IDAir {
				continue
			}
			h := surf.Heights[bx][bz]

			info := Block(id)
			if info == nil {
				continue
			}
			color := info.Color

			// Route water tops into the translucent stream so they match
			// the LOD 0 water alpha (0.55, see defaultBlockAlpha). Solid
			// terrain stays opaque at alpha 1.
			isWater := id == IDWater
			topAlpha := float32(1.0)
			topOut := &opaque
			if isWater {
				topAlpha = 0.55
				topOut = &translucent
			}

			appendSurfaceTop(topOut, float32(baseX+bx), float32(baseX+bx+step),
				float32(h+1),
				float32(baseZ+bz), float32(baseZ+bz+step),
				color, topAlpha)

			// Side skirts towards lower neighbours. Intra-chunk cells look
			// at the adjacent column in the same surface; cells at the
			// chunk boundary consult the neighbouring chunk's surface via
			// the `neighbour` closure so skirts are emitted across chunk
			// boundaries too.
			//
			// Skirts always go into the opaque stream: they're emitted for
			// the current column's terrain face, not for the (potentially
			// water) neighbour below. Water columns sit at sea level and
			// never have a lower neighbour within the same chunk, so they
			// don't produce skirts in practice.
			//
			// +X neighbour.
			if bx+step < 16 {
				nh := surf.Heights[bx+step][bz]
				if nh < h {
					appendSurfaceSideX(&opaque,
						float32(baseX+bx+step),
						float32(nh+1), float32(h+1),
						float32(baseZ+bz), float32(baseZ+bz+step),
						+1, color, 1.0)
				}
			} else if nh, ok := neighbourHeight(1, 0, 0, bz); ok && nh < h {
				appendSurfaceSideX(&opaque,
					float32(baseX+bx+step),
					float32(nh+1), float32(h+1),
					float32(baseZ+bz), float32(baseZ+bz+step),
					+1, color, 1.0)
			}
			// -X neighbour.
			if bx-step >= 0 {
				nh := surf.Heights[bx-step][bz]
				if nh < h {
					appendSurfaceSideX(&opaque,
						float32(baseX+bx),
						float32(nh+1), float32(h+1),
						float32(baseZ+bz), float32(baseZ+bz+step),
						-1, color, 1.0)
				}
			} else if nh, ok := neighbourHeight(-1, 0, 15, bz); ok && nh < h {
				appendSurfaceSideX(&opaque,
					float32(baseX+bx),
					float32(nh+1), float32(h+1),
					float32(baseZ+bz), float32(baseZ+bz+step),
					-1, color, 1.0)
			}
			// +Z neighbour.
			if bz+step < 16 {
				nh := surf.Heights[bx][bz+step]
				if nh < h {
					appendSurfaceSideZ(&opaque,
						float32(baseZ+bz+step),
						float32(nh+1), float32(h+1),
						float32(baseX+bx), float32(baseX+bx+step),
						+1, color, 1.0)
				}
			} else if nh, ok := neighbourHeight(0, 1, bx, 0); ok && nh < h {
				appendSurfaceSideZ(&opaque,
					float32(baseZ+bz+step),
					float32(nh+1), float32(h+1),
					float32(baseX+bx), float32(baseX+bx+step),
					+1, color, 1.0)
			}
			// -Z neighbour.
			if bz-step >= 0 {
				nh := surf.Heights[bx][bz-step]
				if nh < h {
					appendSurfaceSideZ(&opaque,
						float32(baseZ+bz),
						float32(nh+1), float32(h+1),
						float32(baseX+bx), float32(baseX+bx+step),
						-1, color, 1.0)
				}
			} else if nh, ok := neighbourHeight(0, -1, bx, 15); ok && nh < h {
				appendSurfaceSideZ(&opaque,
					float32(baseZ+bz),
					float32(nh+1), float32(h+1),
					float32(baseX+bx), float32(baseX+bx+step),
					-1, color, 1.0)
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
// world X = x. dir > 0 means the quad faces +X (the lower neighbour is on
// the +X side), dir < 0 means it faces -X.
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
// Light attributes are fixed to (0, 0, 0, 1) — no block light, full sky
// light — so distant terrain matches the ambient outdoor look.
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
