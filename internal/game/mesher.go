package game

// Quad is an axis-aligned rectangle produced by greedy meshing. Vertices are
// already in block-local coordinates (0..1) and wound counter-clockwise when
// viewed from outside the block, matching the GL front-face convention.
type Quad struct {
	V      [4][3]float32
	Normal [3]float32
	Color  RGBA
}

// BlockMesh stores the greedy-meshed quads for a single block, grouped by
// face direction. Indices match the Face* constants declared in cube.go.
type BlockMesh struct {
	Faces [6][]Quad
}

// BuildMesh greedy-meshes a 16x16x16 voxel block into a minimal set of quads.
// Internal faces (between two solid voxels) are skipped; coplanar voxels of
// the same palette index are merged into larger rectangles.
func BuildMesh(palette []RGBA, voxels *[PackVoxelCount]uint8) *BlockMesh {
	return &BlockMesh{
		Faces: [6][]Quad{
			meshAxis(palette, voxels, 1, +1), // +Y Top
			meshAxis(palette, voxels, 1, -1), // -Y Bottom
			meshAxis(palette, voxels, 0, +1), // +X Forward
			meshAxis(palette, voxels, 0, -1), // -X Backward
			meshAxis(palette, voxels, 2, +1), // +Z Left
			meshAxis(palette, voxels, 2, -1), // -Z Right
		},
	}
}

func voxelAt(voxels *[PackVoxelCount]uint8, x, y, z int) uint8 {
	if x < 0 || x >= PackBlockSize || y < 0 || y >= PackBlockSize || z < 0 || z >= PackBlockSize {
		return 0
	}
	return voxels[VoxelIndex(x, y, z)]
}

// meshAxis walks each 16x16 slice perpendicular to `axis`, keeps only voxels
// whose neighbour in `dir` is empty, then greedy-merges same-colour rectangles.
func meshAxis(palette []RGBA, voxels *[PackVoxelCount]uint8, axis, dir int) []Quad {
	var quads []Quad

	for s := 0; s < PackBlockSize; s++ {
		var mask [PackBlockSize][PackBlockSize]uint8

		for a := 0; a < PackBlockSize; a++ {
			for b := 0; b < PackBlockSize; b++ {
				x, y, z := sliceCoords(axis, s, a, b)
				v := voxelAt(voxels, x, y, z)
				if v == 0 {
					continue
				}
				nx, ny, nz := x, y, z
				switch axis {
				case 0:
					nx += dir
				case 1:
					ny += dir
				case 2:
					nz += dir
				}
				if voxelAt(voxels, nx, ny, nz) == 0 {
					mask[a][b] = v
				}
			}
		}

		var visited [PackBlockSize][PackBlockSize]bool
		for a := 0; a < PackBlockSize; a++ {
			for b := 0; b < PackBlockSize; b++ {
				if mask[a][b] == 0 || visited[a][b] {
					continue
				}
				col := mask[a][b]

				w := 1
				for b+w < PackBlockSize && mask[a][b+w] == col && !visited[a][b+w] {
					w++
				}

				h := 1
				for a+h < PackBlockSize {
					ok := true
					for db := 0; db < w; db++ {
						if mask[a+h][b+db] != col || visited[a+h][b+db] {
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
						visited[a+da][b+db] = true
					}
				}

				quads = append(quads, buildQuad(s, a, b, h, w, axis, dir, palette[col]))
			}
		}
	}
	return quads
}

// sliceCoords converts a slice position (s) and 2D slice coords (a, b) into
// the 3D voxel coordinate for the given axis.
func sliceCoords(axis, s, a, b int) (int, int, int) {
	switch axis {
	case 0: // slice perpendicular to X: a=Y, b=Z
		return s, a, b
	case 1: // slice perpendicular to Y: a=X, b=Z
		return a, s, b
	case 2: // slice perpendicular to Z: a=X, b=Y
		return a, b, s
	}
	return 0, 0, 0
}

// buildQuad emits a quad for an (a0, b0, h, w) rectangle in the slice at
// depth s, facing `axis`/`dir`. Winding matches drawCube's original order so
// GL back-face culling keeps working.
func buildQuad(s, a0, b0, h, w, axis, dir int, c RGBA) Quad {
	const scale = 1.0 / float32(PackBlockSize)
	fa0 := float32(a0) * scale
	fa1 := float32(a0+h) * scale
	fb0 := float32(b0) * scale
	fb1 := float32(b0+w) * scale
	lo := float32(s) * scale
	hi := float32(s+1) * scale

	q := Quad{Color: c}

	switch {
	case axis == 1 && dir > 0: // +Y: a=X, b=Z
		q.Normal = [3]float32{0, 1, 0}
		q.V = [4][3]float32{
			{fa0, hi, fb0},
			{fa0, hi, fb1},
			{fa1, hi, fb1},
			{fa1, hi, fb0},
		}
	case axis == 1 && dir < 0: // -Y
		q.Normal = [3]float32{0, -1, 0}
		q.V = [4][3]float32{
			{fa0, lo, fb0},
			{fa1, lo, fb0},
			{fa1, lo, fb1},
			{fa0, lo, fb1},
		}
	case axis == 0 && dir > 0: // +X: a=Y, b=Z
		q.Normal = [3]float32{1, 0, 0}
		q.V = [4][3]float32{
			{hi, fa0, fb0},
			{hi, fa1, fb0},
			{hi, fa1, fb1},
			{hi, fa0, fb1},
		}
	case axis == 0 && dir < 0: // -X
		q.Normal = [3]float32{-1, 0, 0}
		q.V = [4][3]float32{
			{lo, fa0, fb0},
			{lo, fa0, fb1},
			{lo, fa1, fb1},
			{lo, fa1, fb0},
		}
	case axis == 2 && dir > 0: // +Z: a=X, b=Y
		q.Normal = [3]float32{0, 0, 1}
		q.V = [4][3]float32{
			{fa0, fb0, hi},
			{fa1, fb0, hi},
			{fa1, fb1, hi},
			{fa0, fb1, hi},
		}
	case axis == 2 && dir < 0: // -Z
		q.Normal = [3]float32{0, 0, -1}
		q.V = [4][3]float32{
			{fa0, fb0, lo},
			{fa0, fb1, lo},
			{fa1, fb1, lo},
			{fa1, fb0, lo},
		}
	}
	return q
}
