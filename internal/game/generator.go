package game

import (
	"math"
	"math/rand"

	"github.com/aquilax/go-perlin"
)

// Terrain generation tuning. All heights are in block units (Y-axis).
const (
	perlinAlpha = 2.0
	perlinBeta  = 2.0
	perlinN     = 3

	seaLevel     = 30 // water fills to this Y
	baseHeight   = 34 // typical grass/beach level
	mountainPeak = 110

	biomeScale  = 260.0 // blocks per unit in biome noise (larger = bigger biomes)
	heightScale = 180.0 // blocks per unit in macro height noise
	detailScale = 40.0  // blocks per unit in small-scale bumps
)

// Biome tags drive surface/sub-surface block selection and feature placement.
type Biome uint8

const (
	BiomePlains Biome = iota
	BiomeForest
	BiomeCherry
	BiomeDesert
	BiomeSnow
	BiomeMountain
	BiomeBeach
	BiomeOcean
)

type terrainGen struct {
	height *perlin.Perlin
	temp   *perlin.Perlin
	humid  *perlin.Perlin
	detail *perlin.Perlin
	seed   int64
}

func newTerrainGen(seed int64) *terrainGen {
	return &terrainGen{
		height: perlin.NewPerlin(perlinAlpha, perlinBeta, perlinN, seed),
		temp:   perlin.NewPerlin(perlinAlpha, perlinBeta, perlinN, seed+1),
		humid:  perlin.NewPerlin(perlinAlpha, perlinBeta, perlinN, seed+2),
		detail: perlin.NewPerlin(perlinAlpha, perlinBeta, perlinN, seed+3),
		seed:   seed,
	}
}

func (g *terrainGen) heightAt(x, z int) int {
	macro := g.height.Noise2D(float64(x)/heightScale, float64(z)/heightScale)
	bump := g.detail.Noise2D(float64(x)/detailScale, float64(z)/detailScale)
	// macro ∈ [-1,1] → span ±40; bump adds ±4 roughness.
	h := baseHeight + int(math.Round(macro*40+bump*4))
	// Clamp to keep terrain inside the world.
	if h < 2 {
		h = 2
	}
	if h > worldHeight-20 {
		h = worldHeight - 20
	}
	return h
}

func (g *terrainGen) temperatureAt(x, z int) float64 {
	return g.temp.Noise2D(float64(x)/biomeScale, float64(z)/biomeScale)
}

func (g *terrainGen) humidityAt(x, z int) float64 {
	return g.humid.Noise2D(float64(x)/biomeScale, float64(z)/biomeScale)
}

// biomeAt maps altitude + climate onto a biome. Ordering matters: ocean and
// beach win over climate biomes when the terrain dips below sea level.
func (g *terrainGen) biomeAt(x, z int) Biome {
	h := g.heightAt(x, z)
	t := g.temperatureAt(x, z)
	humid := g.humidityAt(x, z)

	switch {
	case h < seaLevel-1:
		return BiomeOcean
	case h <= seaLevel+1:
		return BiomeBeach
	case h >= 72:
		return BiomeMountain
	case t < -0.35:
		return BiomeSnow
	case t > 0.3 && humid < -0.1:
		return BiomeDesert
	case t > 0 && humid > 0.25:
		if math.Abs(t-0.1) < 0.15 && humid > 0.35 {
			return BiomeCherry
		}
		return BiomeForest
	}
	return BiomePlains
}

// columnBlocks returns the surface and sub-surface block IDs for the biome.
func columnBlocks(b Biome) (surface, subSurface, deep uint8) {
	switch b {
	case BiomeDesert:
		return IDSand, IDSand, IDStone
	case BiomeBeach:
		return IDSand, IDSand, IDStone
	case BiomeOcean:
		return IDSand, IDDirt, IDStone
	case BiomeSnow:
		return IDSnow, IDDirt, IDStone
	case BiomeMountain:
		return IDStone, IDStone, IDStone
	case BiomeForest, BiomeCherry:
		return IDGrassTop, IDDirt, IDStone
	case BiomePlains:
		return IDGrassTop, IDDirt, IDStone
	}
	return IDGrassTop, IDDirt, IDStone
}

// generateChunk fills the 16x128x16 Chunk with terrain blocks, water and
// surface features (trees) for its biomes.
func (g *terrainGen) generateChunk(c *Chunk) {
	baseX := c.Coordinates.x << 4
	baseZ := c.Coordinates.y << 4

	// Pre-compute height + biome per column so trees can use neighbours.
	var heights [16][16]int
	var biomes [16][16]Biome
	for bx := 0; bx < 16; bx++ {
		for bz := 0; bz < 16; bz++ {
			wx := baseX + bx
			wz := baseZ + bz
			heights[bx][bz] = g.heightAt(wx, wz)
			biomes[bx][bz] = g.biomeAt(wx, wz)
		}
	}

	// Column pass: stone/dirt/grass + water fill.
	for bx := 0; bx < 16; bx++ {
		for bz := 0; bz < 16; bz++ {
			h := heights[bx][bz]
			biome := biomes[bx][bz]
			surface, sub, deep := columnBlocks(biome)

			for y := 0; y <= h; y++ {
				switch {
				case y == h:
					c.Blocks[bx][y][bz] = surface
				case y >= h-3:
					c.Blocks[bx][y][bz] = sub
				default:
					c.Blocks[bx][y][bz] = deep
				}
			}

			// Mountain peaks get a snow cap above the tree line.
			if biome == BiomeMountain && h > 80 {
				for y := h - 1; y <= h; y++ {
					if y >= 0 {
						c.Blocks[bx][y][bz] = IDSnow
					}
				}
			}

			// Fill water up to sea level when terrain dips below.
			if h < seaLevel {
				for y := h + 1; y <= seaLevel && y < worldHeight; y++ {
					c.Blocks[bx][y][bz] = IDWater
				}
			}
		}
	}

	// Feature pass: trees. Deterministic per-chunk RNG so the world is stable.
	rng := rand.New(rand.NewSource(g.seed ^ int64(c.Coordinates.x)*73856093 ^ int64(c.Coordinates.y)*19349663))
	for bx := 2; bx < 14; bx++ {
		for bz := 2; bz < 14; bz++ {
			biome := biomes[bx][bz]
			if biome != BiomeForest && biome != BiomeCherry && biome != BiomePlains {
				continue
			}
			chance := 35
			if biome == BiomeForest {
				chance = 9
			} else if biome == BiomeCherry {
				chance = 14
			}
			if rng.Intn(chance) != 0 {
				continue
			}
			h := heights[bx][bz]
			if h < seaLevel+1 || h > 70 {
				continue
			}
			leaves := IDLeavesOak
			wood := IDWoodOak
			if biome == BiomeCherry {
				leaves = IDLeavesCherry
			} else if biome == BiomeSnow {
				wood = IDWoodPine
			}
			placeTree(c, bx, h+1, bz, wood, leaves, rng)
		}
	}
}

// placeTree stamps a small tree at (bx, baseY, bz) in local chunk coords.
// Trunk height 4-6, leaves as a 3x3 slab with a 1-block top.
func placeTree(c *Chunk, bx, baseY, bz int, wood, leaves uint8, rng *rand.Rand) {
	trunk := 4 + rng.Intn(3)
	if baseY+trunk+2 >= worldHeight {
		return
	}

	for dy := 0; dy < trunk; dy++ {
		c.Blocks[bx][baseY+dy][bz] = wood
	}

	top := baseY + trunk
	for dx := -2; dx <= 2; dx++ {
		for dz := -2; dz <= 2; dz++ {
			if abs(dx) == 2 && abs(dz) == 2 {
				continue
			}
			setLeaf(c, bx+dx, top-1, bz+dz, leaves)
			setLeaf(c, bx+dx, top, bz+dz, leaves)
		}
	}
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			if abs(dx) == 1 && abs(dz) == 1 {
				continue
			}
			setLeaf(c, bx+dx, top+1, bz+dz, leaves)
		}
	}
}

func setLeaf(c *Chunk, bx, y, bz int, id uint8) {
	if bx < 0 || bx >= 16 || bz < 0 || bz >= 16 || y < 0 || y >= worldHeight {
		return
	}
	if c.Blocks[bx][y][bz] == IDAir {
		c.Blocks[bx][y][bz] = id
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
