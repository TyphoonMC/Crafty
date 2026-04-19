package game

import (
	"math"
	"time"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

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
	// Far plane extended to cover the LOD macro ring (lodMacroRadius
	// chunks × 16 blocks ≈ 512 blocks) with comfortable margin so
	// distant meshes don't get clipped at the horizon.
	return mgl32.Perspective(mgl32.DegToRad(70), game.renderer.aspect, 0.1, 4096)
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

// refreshChunkMeshes rebuilds dirty meshes and evicts chunks no longer in
// the active LOD 0 square. Also uploads any missing distant-tier meshes so
// the horizon is populated incrementally as surfaces come in. Must run on
// the GL thread.
func (game *Game) refreshChunkMeshes() {
	r := game.renderer

	active := make(map[Point2D]struct{}, len(game.chunks))
	for coord, c := range game.chunks {
		active[coord] = struct{}{}

		m, ok := r.meshes[coord]
		if !ok {
			m = &chunkMesh{dirty: true}
			r.meshes[coord] = m
		}
		if m.dirty {
			opaque, translucent := BuildChunkMesh(c, game.getBlockAt, game.sampleLight)
			r.uploadMesh(m, opaque, translucent)
		}
	}
	r.evict(active)

	// Distant-tier meshes are pure functions of ChunkSurface, so we just
	// build one for any surface that doesn't already have a mesh.
	game.refreshLODMeshes()
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

	// Pass 1: opaque geometry writes depth normally. LOD 0 chunks render
	// first (they're closest and overwrite distant meshes cheaply thanks to
	// the depth test), then the distant-tier meshes fill in the horizon.
	gl.Disable(gl.BLEND)
	gl.DepthMask(true)
	for _, m := range r.meshes {
		if m.opaqueCount == 0 {
			continue
		}
		gl.BindVertexArray(m.opaqueVAO)
		gl.DrawArrays(gl.TRIANGLES, 0, m.opaqueCount)
	}
	for _, m := range r.lodMeshes {
		if m.opaqueCount == 0 {
			continue
		}
		gl.BindVertexArray(m.opaqueVAO)
		gl.DrawArrays(gl.TRIANGLES, 0, m.opaqueCount)
	}

	// Pass 2: translucent geometry blends over the opaque frame, with depth
	// testing enabled but depth writes off (standard translucent trick so
	// overlapping translucent fragments don't fight each other). LOD 0 water
	// renders first, then distant-tier water so the two blend consistently.
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
	for _, m := range r.lodMeshes {
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
