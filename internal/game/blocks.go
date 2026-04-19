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

// BlockInfo holds the runtime metadata derived from a CRPK block at load time.
// The full voxel grid is kept for future partial-shape rendering; chunk
// meshing only uses Color and flags for now (all default blocks are full
// cubes of a single palette index).
type BlockInfo struct {
	Name        string
	Flags       uint8
	Color       RGBA
	FullCube    bool
	Transparent bool
	Voxels      [PackVoxelCount]uint8
}

var blocks = []*BlockInfo{
	{Name: "air", Transparent: true}, // ID 0
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
	blocks = append(blocks, &BlockInfo{Name: "air", Transparent: true})

	for i := range pack.Blocks {
		bd := &pack.Blocks[i]
		info := &BlockInfo{
			Name:        bd.Name,
			Flags:       bd.Flags,
			Transparent: bd.Flags&BlockFlagTransparent != 0,
			Voxels:      bd.Voxels,
		}
		info.Color, info.FullCube = dominantColor(pack.Palette, &bd.Voxels)
		blocks = append(blocks, info)
	}
	return nil
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
