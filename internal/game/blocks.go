package game

import (
	"fmt"
)

const DefaultPackPath = "./configs/default.zip"

var blockPackPath = DefaultPackPath

// SetBlockPackPath overrides the resource pack path used by LoadBlockPack.
func SetBlockPackPath(path string) {
	blockPackPath = path
}

// AABB is an axis-aligned bounding box in world or block-local coordinates.
type AABB struct {
	MinX, MinY, MinZ, MaxX, MaxY, MaxZ float32
}

// Intersects returns true when a and b overlap on all three axes.
func (a AABB) Intersects(b AABB) bool {
	return a.MinX < b.MaxX && a.MaxX > b.MinX &&
		a.MinY < b.MaxY && a.MaxY > b.MinY &&
		a.MinZ < b.MaxZ && a.MaxZ > b.MinZ
}

// BlockInfo holds the runtime metadata derived from a CRPK block at load time.
// The full voxel grid is kept for future partial-shape rendering; chunk
// meshing only uses Color and flags for now (all default blocks are full
// cubes of a single palette index).
type BlockInfo struct {
	Name           string
	Flags          uint8
	Color          RGBA
	FullCube       bool
	Transparent    bool
	Translucent    bool
	Opaque         bool
	Alpha          float32
	Solid          bool
	CollisionBoxes []AABB
	Voxels         [PackVoxelCount]uint8
	// Lighting metadata. Emissive is derived from BlockFlagEmissive; the
	// LightR/G/B channels hold the 0..15 packed colour emitted by the block.
	Emissive bool
	LightR   uint8
	LightG   uint8
	LightB   uint8
}

// SolidAABBs returns the block-local collision boxes ([0,1]^3) for solid
// blocks, or nil for non-solid blocks.
func (b *BlockInfo) SolidAABBs() []AABB {
	if b == nil || !b.Solid {
		return nil
	}
	return b.CollisionBoxes
}

var blocks = []*BlockInfo{
	{Name: "air", Transparent: true, Alpha: 1}, // ID 0
}

func BlockCount() int { return len(blocks) }

// Block returns the block info for id, or nil if id is out of range.
func Block(id uint8) *BlockInfo {
	if int(id) >= len(blocks) {
		return nil
	}
	return blocks[id]
}

// IsBlockTransparent is a fast path for hot render loops.
func IsBlockTransparent(id uint8) bool {
	if int(id) >= len(blocks) {
		return true
	}
	return blocks[id].Transparent
}

// LoadBlockPack loads the configured .bin/.zip resource pack and populates
// the global block table with per-block colour metadata for chunk meshing.
func LoadBlockPack() error {
	pack, err := LoadPack(blockPackPath)
	if err != nil {
		return fmt.Errorf("load pack %q: %w", blockPackPath, err)
	}

	blocks = make([]*BlockInfo, 0, len(pack.Blocks)+1)
	blocks = append(blocks, &BlockInfo{Name: "air", Transparent: true, Alpha: 1})

	for i := range pack.Blocks {
		bd := &pack.Blocks[i]
		info := &BlockInfo{
			Name:        bd.Name,
			Flags:       bd.Flags,
			Transparent: bd.Flags&BlockFlagTransparent != 0,
			Translucent: bd.Flags&BlockFlagTranslucent != 0,
			Solid:       bd.Flags&BlockFlagSolid != 0,
			Emissive:    bd.Flags&BlockFlagEmissive != 0,
			Voxels:      bd.Voxels,
		}
		info.Opaque = !info.Transparent && !info.Translucent
		info.Alpha = defaultBlockAlpha(info.Name, info.Translucent)
		info.Color, info.FullCube = dominantColor(pack.Palette, &bd.Voxels)
		if info.Solid {
			if info.FullCube {
				info.CollisionBoxes = []AABB{{0, 0, 0, 1, 1, 1}}
			} else {
				info.CollisionBoxes = greedyMergeCollision(&bd.Voxels)
			}
		}
		if info.Emissive {
			info.LightR, info.LightG, info.LightB = emissiveColor(info.Name)
		}
		blocks = append(blocks, info)
	}
	return nil
}

// emissiveColor returns the 0..15 packed RGB light colour emitted by an
// emissive block. Names drive the palette so new emissive blocks can be added
// without touching the engine beyond appending an entry here.
func emissiveColor(name string) (r, g, b uint8) {
	switch name {
	case "lantern_warm":
		return 15, 12, 6
	case "lantern_cool":
		return 6, 10, 15
	case "mushroom_glow":
		return 8, 15, 8
	default:
		return 12, 12, 12
	}
}

// defaultBlockAlpha picks the per-block alpha used during rendering based on
// the CRPK block name. Opaque blocks return 1.0; translucent ones without a
// named override fall back to 0.8.
func defaultBlockAlpha(name string, translucent bool) float32 {
	switch name {
	case "water":
		return 0.55
	case "leaves_oak", "leaves_cherry":
		return 0.7
	}
	if translucent {
		return 0.8
	}
	return 1.0
}

// greedyMergeCollision reduces a block's 16^3 voxel grid to a minimal set of
// block-local AABBs (coords in [0,1]) by greedy 3D merging. Only voxels with
// a non-zero palette index contribute. The algorithm extends along +X first,
// then grows the slab along +Y (requiring each column in the row to match
// the X-extent), then along +Z (requiring each plane in the slab to match
// the X/Y extents). Visited voxels are marked so they aren't re-emitted.
func greedyMergeCollision(voxels *[PackVoxelCount]uint8) []AABB {
	const N = PackBlockSize
	var visited [PackVoxelCount]bool
	out := make([]AABB, 0, 4)

	solid := func(x, y, z int) bool {
		return voxels[VoxelIndex(x, y, z)] != 0
	}

	for z := 0; z < N; z++ {
		for y := 0; y < N; y++ {
			for x := 0; x < N; x++ {
				if visited[VoxelIndex(x, y, z)] || !solid(x, y, z) {
					continue
				}

				// Extend along +X.
				wx := 1
				for x+wx < N && !visited[VoxelIndex(x+wx, y, z)] && solid(x+wx, y, z) {
					wx++
				}

				// Extend along +Y: each candidate row must match the full X-extent.
				wy := 1
				for y+wy < N {
					ok := true
					for dx := 0; dx < wx; dx++ {
						if visited[VoxelIndex(x+dx, y+wy, z)] || !solid(x+dx, y+wy, z) {
							ok = false
							break
						}
					}
					if !ok {
						break
					}
					wy++
				}

				// Extend along +Z: each candidate slab must match the X/Y extents.
				wz := 1
				for z+wz < N {
					ok := true
				slabLoop:
					for dy := 0; dy < wy; dy++ {
						for dx := 0; dx < wx; dx++ {
							if visited[VoxelIndex(x+dx, y+dy, z+wz)] || !solid(x+dx, y+dy, z+wz) {
								ok = false
								break slabLoop
							}
						}
					}
					if !ok {
						break
					}
					wz++
				}

				for dz := 0; dz < wz; dz++ {
					for dy := 0; dy < wy; dy++ {
						for dx := 0; dx < wx; dx++ {
							visited[VoxelIndex(x+dx, y+dy, z+dz)] = true
						}
					}
				}

				out = append(out, AABB{
					MinX: float32(x) / float32(N),
					MinY: float32(y) / float32(N),
					MinZ: float32(z) / float32(N),
					MaxX: float32(x+wx) / float32(N),
					MaxY: float32(y+wy) / float32(N),
					MaxZ: float32(z+wz) / float32(N),
				})
			}
		}
	}
	return out
}

// dominantColor returns the colour to use when chunk-meshing a block as a
// solid cube, plus a flag indicating whether every voxel shares that index
// (the common case for the default pack).
func dominantColor(palette []RGBA, voxels *[PackVoxelCount]uint8) (RGBA, bool) {
	counts := make(map[uint8]int, 4)
	var first uint8
	uniform := true
	for i, v := range voxels {
		if v == 0 {
			continue
		}
		if i == 0 || first == 0 {
			first = v
		} else if v != first {
			uniform = false
		}
		counts[v]++
	}
	if len(counts) == 0 {
		return RGBA{}, false
	}
	var best uint8
	var bestCount int
	for idx, c := range counts {
		if c > bestCount {
			best = idx
			bestCount = c
		}
	}
	if int(best) >= len(palette) {
		return RGBA{}, false
	}
	return palette[best], uniform
}
