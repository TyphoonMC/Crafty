package main

import "github.com/go-gl/gl/v2.1/gl"

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

	mask.top = game.isTransparent(x, y, z, &siblings[0])
	mask.bottom = game.isTransparent(x, y, z, &siblings[1])
	mask.forward = game.isTransparent(x, y, z, &siblings[2])
	mask.backward = game.isTransparent(x, y, z, &siblings[3])
	mask.left = game.isTransparent(x, y, z, &siblings[4])
	mask.right = game.isTransparent(x, y, z, &siblings[5])
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
