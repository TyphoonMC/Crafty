package main

import "github.com/go-gl/gl/v2.1/gl"

const (
	blockTexturePath = "./Rosources/Rosources Texture Pack/assets/minecraft/textures/blocks/"
)

type Block interface {
	Render()
	SetTextureIds([]uint32)
	GetTextureIds() []uint32
	GetTextures() []string
}

type Cube struct {
	texture   string
	textureId uint32
}

func (cube *Cube) Render() {
	gl.BindTexture(gl.TEXTURE_2D, cube.textureId)

	drawCube()
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

type EmptyBlock struct{}

func (empty *EmptyBlock) Render() {}
func (empty *EmptyBlock) GetTextureIds() []uint32 {
	return []uint32{}
}
func (empty *EmptyBlock) SetTextureIds(ids []uint32) {}
func (empty *EmptyBlock) GetTextures() []string {
	return []string{}
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

func (game *Game) drawBlock(x, y, z int, id uint8) {
	if id == 0 {
		return
	}

	b := blocks[id]
	if b != nil {
		gl.PushMatrix()
		gl.Translatef(float32(x), float32(y), float32(z))
		b.Render()
		gl.PopMatrix()
	}
}
