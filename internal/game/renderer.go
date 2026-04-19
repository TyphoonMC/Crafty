package game

import (
	"fmt"
	"unsafe"

	"github.com/go-gl/gl/v3.3-core/gl"
)

const chunkVertexSize = int32(unsafe.Sizeof(ChunkVertex{}))

const vertexShaderSrc = `#version 330 core
layout (location = 0) in vec3 aPos;
layout (location = 1) in vec3 aNormal;
layout (location = 2) in vec3 aColor;

uniform mat4 uMVP;
uniform vec3 uLightDir;
uniform float uAmbient;

out vec3 vColor;
out float vBrightness;

void main() {
    gl_Position = uMVP * vec4(aPos, 1.0);
    float diff = max(dot(normalize(aNormal), normalize(uLightDir)), 0.0);
    vBrightness = uAmbient + (1.0 - uAmbient) * diff;
    vColor = aColor;
}
`

const fragmentShaderSrc = `#version 330 core
in vec3 vColor;
in float vBrightness;

out vec4 FragColor;

void main() {
    FragColor = vec4(vColor * vBrightness, 1.0);
}
`

type chunkMesh struct {
	vao     uint32
	vbo     uint32
	count   int32
	dirty   bool
}

type renderer struct {
	program uint32

	uMVP      int32
	uLightDir int32
	uAmbient  int32

	meshes map[Point2D]*chunkMesh

	viewportW int32
	viewportH int32
	aspect    float32
}

func newRenderer() *renderer {
	return &renderer{
		meshes: make(map[Point2D]*chunkMesh),
		aspect: 16.0 / 9.0,
	}
}

func (r *renderer) init() error {
	prog, err := buildProgram(vertexShaderSrc, fragmentShaderSrc)
	if err != nil {
		return fmt.Errorf("build program: %w", err)
	}
	r.program = prog

	r.uMVP = gl.GetUniformLocation(prog, gl.Str("uMVP\x00"))
	r.uLightDir = gl.GetUniformLocation(prog, gl.Str("uLightDir\x00"))
	r.uAmbient = gl.GetUniformLocation(prog, gl.Str("uAmbient\x00"))

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LEQUAL)
	gl.Enable(gl.CULL_FACE)
	gl.CullFace(gl.BACK)
	gl.FrontFace(gl.CCW)
	gl.ClearColor(0.78, 0.88, 0.96, 1.0)
	return nil
}

func (r *renderer) shutdown() {
	for _, m := range r.meshes {
		r.freeMesh(m)
	}
	r.meshes = nil
	if r.program != 0 {
		gl.DeleteProgram(r.program)
		r.program = 0
	}
}

func (r *renderer) freeMesh(m *chunkMesh) {
	if m.vbo != 0 {
		gl.DeleteBuffers(1, &m.vbo)
	}
	if m.vao != 0 {
		gl.DeleteVertexArrays(1, &m.vao)
	}
}

func (r *renderer) setViewport(w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	r.viewportW = int32(w)
	r.viewportH = int32(h)
	r.aspect = float32(w) / float32(h)
	gl.Viewport(0, 0, r.viewportW, r.viewportH)
}

// markDirty forces a remesh on the next render pass for the given chunk.
func (r *renderer) markDirty(coord Point2D) {
	if m, ok := r.meshes[coord]; ok {
		m.dirty = true
	}
}

// evict removes meshes whose chunks are no longer in the active grid.
func (r *renderer) evict(active map[Point2D]struct{}) {
	for k, m := range r.meshes {
		if _, ok := active[k]; !ok {
			r.freeMesh(m)
			delete(r.meshes, k)
		}
	}
}

func (r *renderer) uploadMesh(m *chunkMesh, verts []ChunkVertex) {
	if m.vao == 0 {
		gl.GenVertexArrays(1, &m.vao)
		gl.GenBuffers(1, &m.vbo)
		gl.BindVertexArray(m.vao)
		gl.BindBuffer(gl.ARRAY_BUFFER, m.vbo)

		gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, chunkVertexSize, 0)
		gl.EnableVertexAttribArray(0)
		gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, chunkVertexSize, 3*4)
		gl.EnableVertexAttribArray(1)
		gl.VertexAttribPointerWithOffset(2, 3, gl.FLOAT, false, chunkVertexSize, 6*4)
		gl.EnableVertexAttribArray(2)
	} else {
		gl.BindVertexArray(m.vao)
		gl.BindBuffer(gl.ARRAY_BUFFER, m.vbo)
	}

	m.count = int32(len(verts))
	if m.count == 0 {
		gl.BindBuffer(gl.ARRAY_BUFFER, 0)
		gl.BindVertexArray(0)
		m.dirty = false
		return
	}

	gl.BufferData(gl.ARRAY_BUFFER, int(m.count)*int(chunkVertexSize), gl.Ptr(verts), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)
	m.dirty = false
}
