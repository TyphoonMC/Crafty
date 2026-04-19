package game

import (
	"math"
	"time"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

const renderRadiusChunks = 2

// InitRenderer must run after the GL context is current. It loads the block
// pack, compiles shaders, and initialises GL state.
func (game *Game) InitRenderer() error {
	if err := LoadBlockPack(); err != nil {
		return err
	}
	r := newRenderer()
	if err := r.init(); err != nil {
		return err
	}
	game.renderer = r
	return nil
}

// ShutdownRenderer releases GPU resources. Safe to call once at exit.
func (game *Game) ShutdownRenderer() {
	if game.renderer != nil {
		game.renderer.shutdown()
		game.renderer = nil
	}
}

// OnFramebufferResize is wired to GLFW's framebuffer size callback.
func (game *Game) OnFramebufferResize(width, height int) {
	if game.renderer != nil {
		game.renderer.setViewport(width, height)
	}
}

func (game *Game) buildProjection() mgl32.Mat4 {
	return mgl32.Perspective(mgl32.DegToRad(70), game.renderer.aspect, 0.1, 1000)
}

// buildView replicates the previous gl.Rotatef / gl.Translatef sequence in
// modelview order so camera behaviour matches the old fixed-function path.
func (game *Game) buildView() mgl32.Mat4 {
	rot := game.player.rot
	pos := game.player.pos

	headBang := float32(math.Sin(float64(pos.x)*5)) * 0.1
	headBang += float32(math.Cos(float64(pos.z)*5)) * 0.1

	view := mgl32.Ident4()
	view = view.Mul4(mgl32.HomogRotate3DX(mgl32.DegToRad(rot.x)))
	view = view.Mul4(mgl32.HomogRotate3DY(mgl32.DegToRad(rot.y)))
	view = view.Mul4(mgl32.HomogRotate3DZ(mgl32.DegToRad(rot.z)))
	view = view.Mul4(mgl32.Translate3D(-pos.x, -(pos.y + 2 + headBang), -pos.z))
	return view
}

// refreshChunkMeshes rebuilds dirty meshes and evicts chunks no longer in the
// active 3x3 grid. Must run on the GL thread.
func (game *Game) refreshChunkMeshes() {
	r := game.renderer

	active := make(map[Point2D]struct{}, 9)
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			c := game.grid[x][y]
			if c == nil {
				continue
			}
			active[c.Coordinates] = struct{}{}

			m, ok := r.meshes[c.Coordinates]
			if !ok {
				m = &chunkMesh{dirty: true}
				r.meshes[c.Coordinates] = m
			}
			if m.dirty {
				opaque, translucent := BuildChunkMesh(c, game.getBlockAt)
				r.uploadMesh(m, opaque, translucent)
			}
		}
	}
	r.evict(active)
}

func (game *Game) drawScene() {
	r := game.renderer
	if r == nil || r.viewportW == 0 {
		return
	}

	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	game.refreshChunkMeshes()

	proj := game.buildProjection()
	view := game.buildView()
	mvp := proj.Mul4(view)

	gl.UseProgram(r.program)
	gl.UniformMatrix4fv(r.uMVP, 1, false, &mvp[0])
	lightDir := mgl32.Vec3{0.5, 1, 0.3}.Normalize()
	gl.Uniform3f(r.uLightDir, lightDir[0], lightDir[1], lightDir[2])
	gl.Uniform1f(r.uAmbient, 0.55)

	// Pass 1: opaque geometry writes depth normally.
	gl.Disable(gl.BLEND)
	gl.DepthMask(true)
	for _, m := range r.meshes {
		if m.opaqueCount == 0 {
			continue
		}
		gl.BindVertexArray(m.opaqueVAO)
		gl.DrawArrays(gl.TRIANGLES, 0, m.opaqueCount)
	}

	// Pass 2: translucent geometry blends over the opaque frame, with depth
	// testing enabled but depth writes off (standard translucent trick so
	// overlapping translucent fragments don't fight each other).
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.DepthMask(false)
	for _, m := range r.meshes {
		if m.translucentCount == 0 {
			continue
		}
		gl.BindVertexArray(m.translucentVAO)
		gl.DrawArrays(gl.TRIANGLES, 0, m.translucentCount)
	}
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)

	gl.BindVertexArray(0)
}

const targetFrameNanos = 16_666_666

// MainLoop runs one frame: physics + input + draw. Serialized against
// concurrent server mutations via the game mutex.
func (game *Game) MainLoop() {
	s := time.Now().UnixNano()

	game.mu.Lock()
	game.inputLoop()
	game.updatePhysics()
	game.drawScene()
	game.mu.Unlock()

	nano := time.Now().UnixNano() - s
	if nano < targetFrameNanos {
		time.Sleep(time.Duration(targetFrameNanos-nano) * time.Nanosecond)
	}
}
