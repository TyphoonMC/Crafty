package game

// ChunkVertex is one vertex of the chunk mesh. Layout: 9 floats, 36 bytes.
type ChunkVertex struct {
	X, Y, Z    float32
	Nx, Ny, Nz float32
	R, G, B    float32
}

// BuildChunkMesh greedy-meshes a chunk using `blockAt` for neighbour queries
// (which seamlessly handles cross-chunk boundaries). All default blocks are
// currently rendered as full cubes in their dominant palette colour.
func BuildChunkMesh(chunk *Chunk, blockAt func(x, y, z int) uint8) []ChunkVertex {
	baseX := chunk.Coordinates.x << 4
	baseZ := chunk.Coordinates.y << 4

	const maxPlane = worldHeight * 16 // largest slice plane area
	mask := make([]uint8, maxPlane)
	visited := make([]bool, maxPlane)

	verts := make([]ChunkVertex, 0, 2048)

	for face := 0; face < 6; face++ {
		axis, sliceCount, aMax, bMax := faceDims(face)
		dir := faceOffsets[face]

		for s := 0; s < sliceCount; s++ {
			area := aMax * bMax
			for i := 0; i < area; i++ {
				mask[i] = 0
				visited[i] = false
			}

			for a := 0; a < aMax; a++ {
				for b := 0; b < bMax; b++ {
					lx, ly, lz := meshLocalCoords(axis, s, a, b)
					id := blockAt(baseX+lx, ly, baseZ+lz)
					if IsBlockTransparent(id) {
						continue
					}
					if !IsBlockTransparent(blockAt(baseX+lx+dir.x, ly+dir.y, baseZ+lz+dir.z)) {
						continue
					}
					mask[a*bMax+b] = id
				}
			}

			for a := 0; a < aMax; a++ {
				for b := 0; b < bMax; b++ {
					idx := a*bMax + b
					if mask[idx] == 0 || visited[idx] {
						continue
					}
					id := mask[idx]

					w := 1
					for b+w < bMax {
						i := a*bMax + b + w
						if mask[i] != id || visited[i] {
							break
						}
						w++
					}

					h := 1
					for a+h < aMax {
						ok := true
						for db := 0; db < w; db++ {
							i := (a+h)*bMax + b + db
							if mask[i] != id || visited[i] {
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
					verts = appendQuad(verts, face, s, a, b, h, w, baseX, baseZ, info.Color)
				}
			}
		}
	}
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
// preserving CCW winding from the outside of the block.
func appendQuad(verts []ChunkVertex, face, s, a0, b0, h, w, baseX, baseZ int, color RGBA) []ChunkVertex {
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
			R: r, G: g, B: bcol,
		}
	}
	verts = append(verts, mk(v[0]), mk(v[1]), mk(v[2]))
	verts = append(verts, mk(v[0]), mk(v[2]), mk(v[3]))
	return verts
}
