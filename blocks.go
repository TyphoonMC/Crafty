package main

import (
	"github.com/go-gl/gl/v2.1/gl"
)

const (
	blockTexturePath = "./Rosources/Rosources Texture Pack/assets/minecraft/textures/blocks/"
)

type Block interface {
	Render(*FaceMask)
	SetTextureIds([]uint32)
	GetTextureIds() []uint32
	GetTextures() []string
	IsTransparent() bool
}

type Cube struct {
	texture   string
	textureId uint32
}

func (cube *Cube) Render(mask *FaceMask) {
	gl.BindTexture(gl.TEXTURE_2D, cube.textureId)

	drawCube(mask)
}
func (cube *Cube) GetTextureIds() []uint32 {
	return []uint32{cube.textureId}
}
func (cube *Cube) SetTextureIds(ids []uint32) {
	cube.textureId = ids[0]
}
func (cube *Cube) GetTextures() []string {
	return []string{cube.texture}
}
func (cube *Cube) IsTransparent() bool {
	return false
}

type EmptyBlock struct{}

func (empty *EmptyBlock) Render(mask *FaceMask) {}
func (empty *EmptyBlock) GetTextureIds() []uint32 {
	return []uint32{}
}
func (empty *EmptyBlock) SetTextureIds(ids []uint32) {}
func (empty *EmptyBlock) GetTextures() []string {
	return []string{}
}
func (empty *EmptyBlock) IsTransparent() bool {
	return true
}

var (
	blocks = []Block{
		&EmptyBlock{},
		&Cube{"stone", 0},
		&Cube{"cobblestone", 0},
		&Cube{"dirt", 0},
		&Cube{"grass_top", 0},
		&Cube{"debug", 0},
	}
)

func loadBlockTextures() {
	for _, block := range blocks {
		textures := block.GetTextures()
		ids := make([]uint32, len(textures))
		for i := range textures {
			ids[i] = newTexture(blockTexturePath + textures[i] + ".png")
		}
		block.SetTextureIds(ids)
	}
}

func unloadBlockTextures() {
	for _, block := range blocks {
		ids := block.GetTextureIds()
		if len(ids) > 0 {
			gl.DeleteTextures(int32(len(ids)), &ids[0])
		}
	}
}

func (game *Game) drawBlock(x, y, z int, id uint8, mask *FaceMask) {
	b := blocks[id]
	if b != nil {
		gl.PushMatrix()
		gl.Translatef(float32(x), float32(y), float32(z))
		b.Render(mask)
		gl.PopMatrix()
	}
}
