// packgen builds the default CRPK resource pack shipped with Crafty.
// Default blocks are either uniform 16^3 voxel cubes filled with a single
// palette colour, or voxel-patterned shapes (round trunks, carved leaves)
// evaluated per-voxel. Run `go run ./cmd/packgen -o configs/default.zip`.
package main

import (
	"flag"
	"log"

	"github.com/TyphoonMC/Crafty/internal/game"
)

type blockSpec struct {
	name    string
	color   game.RGBA
	flags   uint8
	pattern func(x, y, z int) bool // nil = full cube (every voxel solid)
}

// Order matters: IDs are 1-indexed (0 = air) and referenced by the terrain
// generator constants in internal/game/blocks_ids.go. Do not reorder.
var defaultBlocks = []blockSpec{
	{name: "stone", color: rgb(0xB0, 0xB5, 0xBD), flags: game.BlockFlagSolid},                                 // 1
	{name: "cobblestone", color: rgb(0x88, 0x8C, 0x94), flags: game.BlockFlagSolid},                           // 2
	{name: "dirt", color: rgb(0xB8, 0x95, 0x6A), flags: game.BlockFlagSolid},                                  // 3
	{name: "grass_top", color: rgb(0xA8, 0xD8, 0x8A), flags: game.BlockFlagSolid},                             // 4
	{name: "debug", color: rgb(0xFF, 0x00, 0xFF), flags: game.BlockFlagSolid},                                 // 5
	{name: "sand", color: rgb(0xE8, 0xD5, 0xA8), flags: game.BlockFlagSolid},                                  // 6
	{name: "water", color: rgb(0x9B, 0xC4, 0xE2), flags: game.BlockFlagTranslucent},                           // 7
	{name: "snow", color: rgb(0xF0, 0xEE, 0xE8), flags: game.BlockFlagSolid},                                  // 8
	{name: "wood_oak", color: rgb(0xC9, 0xA5, 0x74), flags: game.BlockFlagSolid, pattern: roundedTrunkPattern()},  // 9
	{name: "wood_pine", color: rgb(0xA5, 0x81, 0x68), flags: game.BlockFlagSolid, pattern: roundedTrunkPattern()}, // 10
	// Leaves are opaque per voxel (hard pixel holes). No BlockFlagTranslucent:
	// each solid voxel is a tiny opaque cube, the empty cells literally let
	// light and sky through.
	{name: "leaves_oak", color: rgb(0x96, 0xB8, 0x7A), flags: game.BlockFlagSolid, pattern: carvedLeavesPattern()},    // 11
	{name: "leaves_cherry", color: rgb(0xF4, 0xC2, 0xD0), flags: game.BlockFlagSolid, pattern: carvedLeavesPattern()}, // 12
	{name: "stone_mossy", color: rgb(0x88, 0xA0, 0x6C), flags: game.BlockFlagSolid},                                   // 13
	{name: "clay", color: rgb(0xD4, 0xA0, 0x88), flags: game.BlockFlagSolid},                                          // 14
	{name: "lantern_warm", color: rgb(0xF5, 0xC6, 0x74), flags: game.BlockFlagSolid | game.BlockFlagEmissive},         // 15
	{name: "lantern_cool", color: rgb(0xA5, 0xC8, 0xF0), flags: game.BlockFlagSolid | game.BlockFlagEmissive},         // 16
	{name: "mushroom_glow", color: rgb(0xB8, 0xE8, 0xB0), flags: game.BlockFlagSolid | game.BlockFlagEmissive},        // 17
}

func rgb(r, g, b uint8) game.RGBA {
	return game.RGBA{R: r, G: g, B: b, A: 255}
}

// roundedTrunkPattern produces a cylindrical cross-section centred on
// (7.5, 7.5) in the block's XZ plane, running the full height (y=0..15).
// Radius is ~6.5/16 so the trunk visibly rounds at the corners without
// shrinking the silhouette too much.
func roundedTrunkPattern() func(x, y, z int) bool {
	return func(x, y, z int) bool {
		dx := float32(x) - 7.5
		dz := float32(z) - 7.5
		return dx*dx+dz*dz <= 6.5*6.5
	}
}

// carvedLeavesPattern is a deterministic pseudo-random ~78% density pattern
// using an integer hash of (x, y, z). The hash is global (not block-local)
// so seams between neighbouring leaves blocks don't repeat, giving the
// foliage a natural sparse-but-contiguous look.
func carvedLeavesPattern() func(x, y, z int) bool {
	return func(x, y, z int) bool {
		h := uint32(x)*73856093 ^ uint32(y)*19349663 ^ uint32(z)*83492791
		h ^= h >> 13
		h *= 0x5bd1e995
		h ^= h >> 15
		return (h & 0xFF) < 200 // ~78% density
	}
}

func main() {
	out := flag.String("o", "configs/default.zip", "output pack path")
	flag.Parse()

	pack := &game.Pack{
		Palette: make([]game.RGBA, 0, len(defaultBlocks)+1),
		Blocks:  make([]game.BlockDef, 0, len(defaultBlocks)),
	}
	pack.Palette = append(pack.Palette, game.RGBA{}) // index 0: air

	for _, s := range defaultBlocks {
		paletteIdx := uint8(len(pack.Palette))
		pack.Palette = append(pack.Palette, s.color)

		var bd game.BlockDef
		bd.Name = s.name
		bd.Flags = s.flags
		if s.pattern == nil {
			for i := range bd.Voxels {
				bd.Voxels[i] = paletteIdx
			}
		} else {
			for x := 0; x < game.PackBlockSize; x++ {
				for y := 0; y < game.PackBlockSize; y++ {
					for z := 0; z < game.PackBlockSize; z++ {
						if s.pattern(x, y, z) {
							bd.Voxels[game.VoxelIndex(x, y, z)] = paletteIdx
						}
					}
				}
			}
		}
		pack.Blocks = append(pack.Blocks, bd)
	}

	if err := game.WritePackZip(*out, pack); err != nil {
		log.Fatalf("write pack: %v", err)
	}
	log.Printf("wrote %d blocks, %d palette entries to %s", len(pack.Blocks), len(pack.Palette), *out)
}
