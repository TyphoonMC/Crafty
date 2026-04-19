package game

// ChunkVertex is one vertex of the chunk mesh. Layout: 14 floats, 56 bytes.
// The trailing A channel lets the translucent pass shade water/leaves with
// the per-block alpha loaded from the resource pack. LR/LG/LB carry the
// coloured block light (0..1) and LSky carries the sky light (0..1) sampled
// from the AIR-side voxel adjacent to each face.
type ChunkVertex struct {
	X, Y, Z    float32
	Nx, Ny, Nz float32
	R, G, B, A float32
	LR, LG, LB float32
	LSky       float32
}

// blockIsOpaque reports whether a neighbour of the given id should fully
// occlude a face. Out-of-table ids behave like air (not opaque). Kept as a
// hot-path helper for the greedy mesher below.
func blockIsOpaque(id uint8) bool {
	if int(id) >= len(blocks) {
		return false
	}
	return blocks[id].Opaque
}

// blockIsTranslucent reports whether the block draws geometry with alpha < 1.
func blockIsTranslucent(id uint8) bool {
	if int(id) >= len(blocks) {
		return false
	}
	return blocks[id].Translucent
}

// faceVisible decides whether the face of block A (id `a`) pointing at
// neighbour B (id `b`) should be emitted, and whether it belongs in the opaque
// or translucent bucket. Encodes the rule table from the task brief:
//
//   - A transparent: never draw (caller already filters)
//   - B transparent / out-of-world: draw
//   - A opaque, B opaque: cull
//   - A opaque, B translucent: draw to opaque
//   - A translucent, B opaque: cull
//   - A translucent, B translucent, same id: cull (shared inner surface)
//   - A translucent, B translucent, different id: draw to translucent
func faceVisible(a, b uint8) (draw bool, translucent bool) {
	aInfo := Block(a)
	if aInfo == nil || aInfo.Transparent {
		return false, false
	}
	aTrans := aInfo.Translucent
	aOpaque := aInfo.Opaque

	// Neighbour transparent (air / glass-like) or out of the world.
	if IsBlockTransparent(b) {
		return true, aTrans
	}

	bOpaque := blockIsOpaque(b)
	bTrans := blockIsTranslucent(b)

	if aOpaque {
		if bOpaque {
			return false, false
		}
		// Neighbour translucent → draw A's opaque face.
		return true, false
	}

	// A translucent.
	if bOpaque {
		return false, false
	}
	if bTrans && a == b {
		return false, false
	}
	return true, true
}

// BuildChunkMesh greedy-meshes a chunk and returns two vertex slices: the
// opaque geometry (drawn with depth write) and the translucent geometry (drawn
// afterwards with blending). `blockAt` seamlessly handles cross-chunk
// neighbour queries. `lightAt` is sampled once per merged rectangle on the
// air-side voxel adjacent to the face centre to colour the vertices.
func BuildChunkMesh(
	chunk *Chunk,
	blockAt func(x, y, z int) uint8,
	lightAt func(x, y, z int) (r, g, b, sky float32),
) (opaque, translucent []ChunkVertex) {
	baseX := chunk.Coordinates.x << 4
	baseZ := chunk.Coordinates.y << 4

	const maxPlane = worldHeight * 16 // largest slice plane area
	mask := make([]uint8, maxPlane)
	maskTrans := make([]bool, maxPlane)
	visited := make([]bool, maxPlane)

	opaque = make([]ChunkVertex, 0, 2048)
	translucent = make([]ChunkVertex, 0, 256)

	for face := 0; face < 6; face++ {
		axis, sliceCount, aMax, bMax := faceDims(face)
		dir := faceOffsets[face]

		for s := 0; s < sliceCount; s++ {
			area := aMax * bMax
			for i := 0; i < area; i++ {
				mask[i] = 0
				maskTrans[i] = false
				visited[i] = false
			}

			for a := 0; a < aMax; a++ {
				for b := 0; b < bMax; b++ {
					lx, ly, lz := meshLocalCoords(axis, s, a, b)
					id := blockAt(baseX+lx, ly, baseZ+lz)
					// Non-full-cube blocks (carved leaves, round trunks, etc.)
					// are emitted by the per-block pass below; skip them here
					// so the greedy cube mesher doesn't draw them as full cubes.
					if info := Block(id); info == nil || !info.FullCube {
						continue
					}
					nid := blockAt(baseX+lx+dir.x, ly+dir.y, baseZ+lz+dir.z)

					draw, trans := faceVisible(id, nid)
					if !draw {
						continue
					}
					mask[a*bMax+b] = id
					maskTrans[a*bMax+b] = trans
				}
			}

			for a := 0; a < aMax; a++ {
				for b := 0; b < bMax; b++ {
					idx := a*bMax + b
					if mask[idx] == 0 || visited[idx] {
						continue
					}
					id := mask[idx]
					trans := maskTrans[idx]

					w := 1
					for b+w < bMax {
						i := a*bMax + b + w
						if mask[i] != id || maskTrans[i] != trans || visited[i] {
							break
						}
						w++
					}

					h := 1
					for a+h < aMax {
						ok := true
						for db := 0; db < w; db++ {
							i := (a+h)*bMax + b + db
							if mask[i] != id || maskTrans[i] != trans || visited[i] {
								ok = false
								break
							}
						}
						if !ok {
							break
						}
						h++
					}

					for da := 0; da < h; da++ {
						for db := 0; db < w; db++ {
							visited[(a+da)*bMax+b+db] = true
						}
					}

					info := Block(id)
					if info == nil {
						continue
					}

					// Sample light at the centre of the merged rectangle on
					// the air-side neighbour voxel. Integer division gives a
					// reasonable approximation that avoids floating-point in
					// the hot path.
					ca := a + h/2
					cb := b + w/2
					clx, cly, clz := meshLocalCoords(axis, s, ca, cb)
					lr, lg, lb, lsky := lightAt(
						baseX+clx+dir.x,
						cly+dir.y,
						baseZ+clz+dir.z,
					)

					if trans {
						translucent = appendQuad(translucent, face, s, a, b, h, w, baseX, baseZ, info.Color, info.Alpha, lr, lg, lb, lsky)
					} else {
						opaque = appendQuad(opaque, face, s, a, b, h, w, baseX, baseZ, info.Color, 1.0, lr, lg, lb, lsky)
					}
				}
			}
		}
	}

	// Second pass: emit pre-built per-block voxel meshes for non-full-cube
	// blocks (carved leaves, round trunks, ...). The population of such blocks
	// per chunk is tiny (tree trunks + leaves), so an O(n) scan is cheap.
	for bx := 0; bx < 16; bx++ {
		for by := 0; by < worldHeight; by++ {
			for bz := 0; bz < 16; bz++ {
				wx, wy, wz := baseX+bx, by, baseZ+bz
				id := blockAt(wx, wy, wz)
				if id == 0 {
					continue
				}
				info := Block(id)
				if info == nil || info.FullCube || info.Transparent || info.PerBlockMesh == nil {
					continue
				}
				opaque, translucent = appendPerBlockQuads(opaque, translucent, info, id, wx, wy, wz, blockAt, lightAt)
			}
		}
	}
	return opaque, translucent
}

// appendPerBlockQuads emits the per-voxel greedy quads for a non-full-cube
// block at world position (wx, wy, wz). For each of the six face directions,
// entire faces are hidden when the neighbour fully occludes them (full-cube
// opaque neighbour, or an adjacent identical non-full-cube block).
func appendPerBlockQuads(
	opaque, translucent []ChunkVertex,
	info *BlockInfo,
	id uint8,
	wx, wy, wz int,
	blockAt func(x, y, z int) uint8,
	lightAt func(x, y, z int) (r, g, b, sky float32),
) ([]ChunkVertex, []ChunkVertex) {
	mesh := info.PerBlockMesh
	alpha := info.Alpha
	trans := info.Translucent

	for dir := 0; dir < 6; dir++ {
		quads := mesh.Faces[dir]
		if len(quads) == 0 {
			continue
		}
		off := faceOffsets[dir]
		nx, ny, nz := wx+off.x, wy+off.y, wz+off.z
		nid := blockAt(nx, ny, nz)
		// Hide faces completely occluded by the neighbour.
		if nid != 0 {
			if nInfo := Block(nid); nInfo != nil {
				// Full-cube opaque neighbour: entire face direction is hidden.
				if nInfo.Opaque {
					continue
				}
				// Identical non-full-cube neighbour (e.g. stacked trunks):
				// skip this face direction to avoid double-rendering the
				// shared inner surface.
				if nid == id {
					continue
				}
			}
		}

		lr, lg, lb, lsky := lightAt(nx, ny, nz)
		for i := range quads {
			if trans {
				translucent = appendPerBlockQuadTris(translucent, &quads[i], wx, wy, wz, alpha, lr, lg, lb, lsky)
			} else {
				opaque = appendPerBlockQuadTris(opaque, &quads[i], wx, wy, wz, 1.0, lr, lg, lb, lsky)
			}
		}
	}
	return opaque, translucent
}

// appendPerBlockQuadTris converts a block-local Quad into two ChunkVertex
// triangles, translating the [0,1] block-local vertex coordinates by the
// block's world position and tagging them with the supplied alpha/light.
func appendPerBlockQuadTris(
	verts []ChunkVertex,
	q *Quad,
	wx, wy, wz int,
	alpha, lr, lg, lb, lsky float32,
) []ChunkVertex {
	bx := float32(wx)
	by := float32(wy)
	bz := float32(wz)
	r := float32(q.Color.R) / 255
	g := float32(q.Color.G) / 255
	bcol := float32(q.Color.B) / 255
	nx, ny, nz := q.Normal[0], q.Normal[1], q.Normal[2]

	mk := func(p [3]float32) ChunkVertex {
		return ChunkVertex{
			X: bx + p[0], Y: by + p[1], Z: bz + p[2],
			Nx: nx, Ny: ny, Nz: nz,
			R: r, G: g, B: bcol, A: alpha,
			LR: lr, LG: lg, LB: lb, LSky: lsky,
		}
	}
	verts = append(verts, mk(q.V[0]), mk(q.V[1]), mk(q.V[2]))
	verts = append(verts, mk(q.V[0]), mk(q.V[2]), mk(q.V[3]))
	return verts
}

// faceDims returns the sweep axis, slice count and 2D plane dimensions (aMax,
// bMax) for the given face.
func faceDims(face int) (axis, sliceCount, aMax, bMax int) {
	switch face {
	case FaceTop, FaceBottom:
		return 1, worldHeight, 16, 16 // a=X, b=Z
	case FaceForward, FaceBackward:
		return 0, 16, worldHeight, 16 // a=Y, b=Z
	case FaceLeft, FaceRight:
		return 2, 16, 16, worldHeight // a=X, b=Y
	}
	return 0, 0, 0, 0
}

func meshLocalCoords(axis, s, a, b int) (x, y, z int) {
	switch axis {
	case 0:
		return s, a, b
	case 1:
		return a, s, b
	case 2:
		return a, b, s
	}
	return 0, 0, 0
}

// appendQuad emits two triangles (6 vertices) for a greedy-merged rectangle,
// preserving CCW winding from the outside of the block. The per-vertex alpha
// comes from the block's runtime metadata (1.0 for opaque, <1 for translucent).
// lr/lg/lb/lsky are the normalized block/sky light sampled once for the whole
// rectangle at the face centre.
func appendQuad(verts []ChunkVertex, face, s, a0, b0, h, w, baseX, baseZ int, color RGBA, alpha, lr, lg, lb, lsky float32) []ChunkVertex {
	fa0 := float32(a0)
	fa1 := float32(a0 + h)
	fb0 := float32(b0)
	fb1 := float32(b0 + w)
	lo := float32(s)
	hi := float32(s + 1)
	bx := float32(baseX)
	bz := float32(baseZ)

	var v [4][3]float32
	var n [3]float32

	switch face {
	case FaceTop: // +Y, a=X, b=Z
		n = [3]float32{0, 1, 0}
		v = [4][3]float32{
			{bx + fa0, hi, bz + fb0},
			{bx + fa0, hi, bz + fb1},
			{bx + fa1, hi, bz + fb1},
			{bx + fa1, hi, bz + fb0},
		}
	case FaceBottom: // -Y
		n = [3]float32{0, -1, 0}
		v = [4][3]float32{
			{bx + fa0, lo, bz + fb0},
			{bx + fa1, lo, bz + fb0},
			{bx + fa1, lo, bz + fb1},
			{bx + fa0, lo, bz + fb1},
		}
	case FaceForward: // +X, a=Y, b=Z
		n = [3]float32{1, 0, 0}
		v = [4][3]float32{
			{bx + hi, fa0, bz + fb0},
			{bx + hi, fa1, bz + fb0},
			{bx + hi, fa1, bz + fb1},
			{bx + hi, fa0, bz + fb1},
		}
	case FaceBackward: // -X
		n = [3]float32{-1, 0, 0}
		v = [4][3]float32{
			{bx + lo, fa0, bz + fb0},
			{bx + lo, fa0, bz + fb1},
			{bx + lo, fa1, bz + fb1},
			{bx + lo, fa1, bz + fb0},
		}
	case FaceLeft: // +Z, a=X, b=Y
		n = [3]float32{0, 0, 1}
		v = [4][3]float32{
			{bx + fa0, fb0, bz + hi},
			{bx + fa1, fb0, bz + hi},
			{bx + fa1, fb1, bz + hi},
			{bx + fa0, fb1, bz + hi},
		}
	case FaceRight: // -Z
		n = [3]float32{0, 0, -1}
		v = [4][3]float32{
			{bx + fa0, fb0, bz + lo},
			{bx + fa0, fb1, bz + lo},
			{bx + fa1, fb1, bz + lo},
			{bx + fa1, fb0, bz + lo},
		}
	}

	r := float32(color.R) / 255
	g := float32(color.G) / 255
	bcol := float32(color.B) / 255
	mk := func(p [3]float32) ChunkVertex {
		return ChunkVertex{
			X: p[0], Y: p[1], Z: p[2],
			Nx: n[0], Ny: n[1], Nz: n[2],
			R: r, G: g, B: bcol, A: alpha,
			LR: lr, LG: lg, LB: lb, LSky: lsky,
		}
	}
	verts = append(verts, mk(v[0]), mk(v[1]), mk(v[2]))
	verts = append(verts, mk(v[0]), mk(v[2]), mk(v[3]))
	return verts
}
