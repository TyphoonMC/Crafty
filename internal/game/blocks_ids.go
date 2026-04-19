package game

// Canonical block IDs matching the order emitted by cmd/packgen.
// Keep in sync with cmd/packgen/main.go:defaultBlocks.
const (
	IDAir          uint8 = 0
	IDStone        uint8 = 1
	IDCobblestone  uint8 = 2
	IDDirt         uint8 = 3
	IDGrassTop     uint8 = 4
	IDDebug        uint8 = 5
	IDSand         uint8 = 6
	IDWater        uint8 = 7
	IDSnow         uint8 = 8
	IDWoodOak      uint8 = 9
	IDWoodPine     uint8 = 10
	IDLeavesOak    uint8 = 11
	IDLeavesCherry uint8 = 12
	IDStoneMossy   uint8 = 13
	IDClay         uint8 = 14
)
