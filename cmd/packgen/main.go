// packgen builds the default CRPK resource pack shipped with Crafty.
// Each default block is a solid 16^3 voxel cube filled with a single palette
// colour. Run `go run ./cmd/packgen -o configs/default.zip`.
package main

import (
	"flag"
	"log"

	"github.com/TyphoonMC/Crafty/internal/game"
)

type blockSpec struct {
	name  string
	color game.RGBA
	flags uint8
}

// Order matters: IDs are 1-indexed (0 = air) and referenced by the terrain
// generator constants in internal/game/blocks_ids.go. Do not reorder.
var defaultBlocks = []blockSpec{
	{name: "stone", color: rgb(0xB0, 0xB5, 0xBD), flags: game.BlockFlagSolid},        // 1
	{name: "cobblestone", color: rgb(0x88, 0x8C, 0x94), flags: game.BlockFlagSolid},  // 2
	{name: "dirt", color: rgb(0xB8, 0x95, 0x6A), flags: game.BlockFlagSolid},         // 3
	{name: "grass_top", color: rgb(0xA8, 0xD8, 0x8A), flags: game.BlockFlagSolid},    // 4
	{name: "debug", color: rgb(0xFF, 0x00, 0xFF), flags: game.BlockFlagSolid},        // 5
	{name: "sand", color: rgb(0xE8, 0xD5, 0xA8), flags: game.BlockFlagSolid},         // 6
	{name: "water", color: rgb(0x9B, 0xC4, 0xE2), flags: game.BlockFlagSolid},        // 7
	{name: "snow", color: rgb(0xF0, 0xEE, 0xE8), flags: game.BlockFlagSolid},         // 8
	{name: "wood_oak", color: rgb(0xC9, 0xA5, 0x74), flags: game.BlockFlagSolid},     // 9
	{name: "wood_pine", color: rgb(0xA5, 0x81, 0x68), flags: game.BlockFlagSolid},    // 10
	{name: "leaves_oak", color: rgb(0x96, 0xB8, 0x7A), flags: game.BlockFlagSolid},   // 11
	{name: "leaves_cherry", color: rgb(0xF4, 0xC2, 0xD0), flags: game.BlockFlagSolid}, // 12
	{name: "stone_mossy", color: rgb(0x88, 0xA0, 0x6C), flags: game.BlockFlagSolid},  // 13
	{name: "clay", color: rgb(0xD4, 0xA0, 0x88), flags: game.BlockFlagSolid},         // 14
}

func rgb(r, g, b uint8) game.RGBA {
	return game.RGBA{R: r, G: g, B: b, A: 255}
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
		for i := range bd.Voxels {
			bd.Voxels[i] = paletteIdx
		}
		pack.Blocks = append(pack.Blocks, bd)
	}

	if err := game.WritePackZip(*out, pack); err != nil {
		log.Fatalf("write pack: %v", err)
	}
	log.Printf("wrote %d blocks, %d palette entries to %s", len(pack.Blocks), len(pack.Palette), *out)
}
