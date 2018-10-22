package main

import (
	"github.com/go-gl/gl/v2.1/gl"
	_ "unsafe"
)

type FaceMask struct {
	left, right, forward, backward, top, bottom bool
}

var (
	siblings = []Point3D{
		{0, 1, 0},
		{0, -1, 0},
		{1, 0, 0},
		{-1, 0, 0},
		{0, 0, 1},
		{0, 0, -1},
	}
)

func (game *Game) isTransparent(bx, by, bz int, face *Point3D) bool {
	id := game.getBlockAt(bx+face.x, by+face.y, bz+face.z)
	return blocks[id].IsTransparent()
}

func (game *Game) calculateMask(x, y, z int, mask *FaceMask) {
	id := game.getBlockAt(x, y, z)
	if id == 0 {
		return
	}

	/*mask.top = true
	mask.bottom = true
	mask.forward = true
	mask.backward = true
	mask.left = true
	mask.right = true*/

	mask.top = game.isTransparent(x, y, z, &siblings[0])
	mask.bottom = game.isTransparent(x, y, z, &siblings[1])
	mask.forward = game.isTransparent(x, y, z, &siblings[2])
	mask.backward = game.isTransparent(x, y, z, &siblings[3])
	mask.left = game.isTransparent(x, y, z, &siblings[4])
	mask.right = game.isTransparent(x, y, z, &siblings[5])
}

var (
	vertices = []int32{
		1, 1, 1,  -1, 1, 1,  -1,-1, 1,  1,-1, 1, // front
		1, 1, 1,   1,-1, 1,   1,-1,-1,  1, 1,-1, // right
		1, 1, 1,   1, 1,-1,  -1, 1,-1, -1, 1, 1, // top
		-1, 1, 1,  -1, 1,-1,  -1,-1,-1, -1,-1, 1, // left
		-1,-1,-1,   1,-1,-1,   1,-1, 1, -1,-1, 1, // bottom
		1,-1,-1,  -1,-1,-1,  -1, 1,-1,  1, 1,-1,  // back
	}

	normals = []int32{
		0, 0, 1,   0, 0, 1,   0, 0, 1,   0, 0, 1,
		1, 0, 0,   1, 0, 0,   1, 0, 0,   1, 0, 0,
		0, 1, 0,   0, 1, 0,   0, 1, 0,   0, 1, 0,
		-1, 0, 0,  -1, 0, 0,  -1, 0, 0,  -1, 0, 0,
		0,-1, 0,   0,-1, 0,   0,-1, 0,   0,-1, 0,
		0, 0,-1,   0, 0,-1,   0, 0,-1,   0, 0,-1,
	}

	colors = []int32{
		1, 1, 1,   1, 1, 0,   1, 0, 0,   1, 0, 1,
		1, 1, 1,   1, 0, 1,   0, 0, 1,   0, 1, 1,
		1, 1, 1,   0, 1, 1,   0, 1, 0,   1, 1, 0,
		1, 1, 0,   0, 1, 0,   0, 0, 0,   1, 0, 0,
		0, 0, 0,   0, 0, 1,   1, 0, 1,   1, 0, 0,
		0, 0, 1,   0, 0, 0,   0, 1, 0,   0, 1, 1,
	}

	textures = []int32{
		1, 0,   0, 0,   0, 1,   1, 1,
		0, 0,   0, 1,   1, 1,   1, 0,
		1, 1,   1, 0,   0, 0,   0, 1,
		1, 0,   0, 0,   0, 1,   1, 1,
		0, 1,   1, 1,   1, 0,   0, 0,
		0, 1,   1, 1,   1, 0,   0, 0,
	}

	indices = []int32{
		0, 1, 2,   2, 3, 0,
		4, 5, 6,   6, 7, 4,
		8, 9,10,  10,11, 8,
		12,13,14,  14,15,12,
		16,17,18,  18,19,16,
		20,21,22,  22,23,20,
	}

	vboId uint32
	iboId uint32
)

func (game *Game) initVBO() {
	/*gl.GenBuffers(1, &vboId)
	gl.GenBuffers(1, &iboId)

	verticesSize := len(vertices)
	normalsSize := len(vertices)
	colorsSize := len(colors)
	texturesSize := len(textures)

	gl.BindBuffer(gl.ARRAY_BUFFER, vboId)
	gl.BufferData(gl.ARRAY_BUFFER, verticesSize+normalsSize+colorsSize+texturesSize, nil, gl.STATIC_DRAW)
	gl.BufferSubData(gl.ARRAY_BUFFER, 0, verticesSize, unsafe.Pointer(vertices))
	gl.BufferSubData(gl.ARRAY_BUFFER, verticesSize, normalsSize, unsafe.Pointer(&normals))
	gl.BufferSubData(gl.ARRAY_BUFFER, verticesSize+normalsSize, colorsSize, unsafe.Pointer(&colors))
	gl.BufferSubData(gl.ARRAY_BUFFER, verticesSize+normalsSize+colorsSize, texturesSize, unsafe.Pointer(&textures))
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, iboId)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices), unsafe.Pointer(&indices), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)*/
}

func drawCube(mask *FaceMask) {
	gl.Color4f(1, 1, 1, 1)

	gl.Begin(gl.QUADS)

	if mask.left {
		gl.Normal3f(0, 0, 1)
		gl.TexCoord2f(0, 0)
		gl.Vertex3f(0, 0, 1)
		gl.TexCoord2f(1, 0)
		gl.Vertex3f(1, 0, 1)
		gl.TexCoord2f(1, 1)
		gl.Vertex3f(1, 1, 1)
		gl.TexCoord2f(0, 1)
		gl.Vertex3f(0, 1, 1)
	}

	if mask.right {
		gl.Normal3f(0, 0, 0)
		gl.TexCoord2f(1, 0)
		gl.Vertex3f(0, 0, 0)
		gl.TexCoord2f(1, 1)
		gl.Vertex3f(0, 1, 0)
		gl.TexCoord2f(0, 1)
		gl.Vertex3f(1, 1, 0)
		gl.TexCoord2f(0, 0)
		gl.Vertex3f(1, 0, 0)
	}

	if mask.top {
		gl.Normal3f(0, 1, 0)
		gl.TexCoord2f(0, 1)
		gl.Vertex3f(0, 1, 0)
		gl.TexCoord2f(0, 0)
		gl.Vertex3f(0, 1, 1)
		gl.TexCoord2f(1, 0)
		gl.Vertex3f(1, 1, 1)
		gl.TexCoord2f(1, 1)
		gl.Vertex3f(1, 1, 0)
	}

	if mask.bottom {
		gl.Normal3f(0, 0, 0)
		gl.TexCoord2f(1, 1)
		gl.Vertex3f(0, 0, 0)
		gl.TexCoord2f(0, 1)
		gl.Vertex3f(1, 0, 0)
		gl.TexCoord2f(0, 0)
		gl.Vertex3f(1, 0, 1)
		gl.TexCoord2f(1, 0)
		gl.Vertex3f(0, 0, 1)
	}

	if mask.forward {
		gl.Normal3f(1, 0, 0)
		gl.TexCoord2f(1, 0)
		gl.Vertex3f(1, 0, 0)
		gl.TexCoord2f(1, 1)
		gl.Vertex3f(1, 1, 0)
		gl.TexCoord2f(0, 1)
		gl.Vertex3f(1, 1, 1)
		gl.TexCoord2f(0, 0)
		gl.Vertex3f(1, 0, 1)
	}

	if mask.backward {
		gl.Normal3f(0, 0, 0)
		gl.TexCoord2f(0, 0)
		gl.Vertex3f(0, 0, 0)
		gl.TexCoord2f(1, 0)
		gl.Vertex3f(0, 0, 1)
		gl.TexCoord2f(1, 1)
		gl.Vertex3f(0, 1, 1)
		gl.TexCoord2f(0, 1)
		gl.Vertex3f(0, 1, 0)
	}

	gl.End()
}
