package game

import (
	"fmt"
	"sort"
	"unsafe"

	"github.com/go-gl/gl/v3.3-core/gl"
)

const chunkVertexSize = int32(unsafe.Sizeof(ChunkVertex{}))

const vertexShaderSrc = `#version 330 core
layout (location = 0) in vec3 aPos;
layout (location = 1) in vec3 aNormal;
layout (location = 2) in vec4 aColor;
layout (location = 3) in vec4 aLight; // rgb block light + sky

uniform mat4 uMVP;
uniform vec3 uLightDir;
uniform float uAmbient;

out vec4 vColor;
out vec4 vLight;
out float vBrightness;

void main() {
    gl_Position = uMVP * vec4(aPos, 1.0);
    float diff = max(dot(normalize(aNormal), normalize(uLightDir)), 0.0);
    vBrightness = uAmbient + (1.0 - uAmbient) * diff;
    vColor = aColor;
    vLight = aLight;
}
`

const fragmentShaderSrc = `#version 330 core
in vec4 vColor;
in vec4 vLight;
in float vBrightness;

out vec4 FragColor;

void main() {
    // Block light (rgb) added to sky light (warm-tinted white).
    vec3 skyTint = vec3(1.0, 0.96, 0.88);
    vec3 lighting = vLight.rgb + skyTint * vLight.w;
    // Mix directional ambient (vBrightness) with static lighting, never fully
    // dark: minimum 0.12 so pure shadow still shows colour.
    float staticL = max(max(lighting.r, lighting.g), lighting.b);
    vec3 lit = vColor.rgb * max(vBrightness * (0.3 + 0.7 * staticL), 0.12);
    // Coloured tint from block light adds warmth.
    lit += vColor.rgb * lighting * 0.35;
    FragColor = vec4(lit, vColor.a);
}
`

type chunkMesh struct {
	opaqueVAO      uint32
	opaqueVBO      uint32
	opaqueCount    int32
	translucentVAO uint32
	translucentVBO uint32
	translucentCount int32
	dirty          bool
}

// lodMesh is the distant-tier counterpart to chunkMesh. Each instance
// batches a whole LOD sector (edge = lodSectorSize(tier) chunks) into
// one opaque + one translucent draw call. The translucent stream is
// populated at LOD 1/2 (water) and left empty at LOD 3/4.
//
// tier, sectorCoord and lastUsed are bookkeeping for the LOD sector
// cache: tier drives step/size, sectorCoord is the map key, lastUsed
// feeds the LRU eviction pass.
type lodMesh struct {
	tier        int
	sectorCoord Point2D
	lastUsed    uint64

	opaqueVAO   uint32
	opaqueVBO   uint32
	opaqueCount int32

	translucentVAO   uint32
	translucentVBO   uint32
	translucentCount int32
}

// lodMeshKey identifies an LOD sector mesh inside the renderer's cache.
// Two distinct tiers can cover the same chunk (e.g. when the player
// crosses a ring boundary) — keying by both avoids collisions.
type lodMeshKey struct {
	tier   int
	sector Point2D
}

type renderer struct {
	program uint32

	uMVP      int32
	uLightDir int32
	uAmbient  int32

	meshes map[Point2D]*chunkMesh
	// lodMeshes holds the distant-tier sector meshes keyed by
	// (tier, sectorCoord). Populated lazily by streamChunks; the LRU
	// eviction pass trims them back to `lruMeshBudget` entries.
	lodMeshes map[lodMeshKey]*lodMesh
	// lodClock is a monotonically increasing stamp used to age lodMesh
	// entries for LRU eviction.
	lodClock uint64

	viewportW int32
	viewportH int32
	aspect    float32
}

func newRenderer() *renderer {
	return &renderer{
		meshes:    make(map[Point2D]*chunkMesh),
		lodMeshes: make(map[lodMeshKey]*lodMesh),
		aspect:    16.0 / 9.0,
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
	for _, m := range r.lodMeshes {
		r.freeLODMesh(m)
	}
	r.lodMeshes = nil
	if r.program != 0 {
		gl.DeleteProgram(r.program)
		r.program = 0
	}
}

func (r *renderer) freeMesh(m *chunkMesh) {
	if m.opaqueVBO != 0 {
		gl.DeleteBuffers(1, &m.opaqueVBO)
		m.opaqueVBO = 0
	}
	if m.opaqueVAO != 0 {
		gl.DeleteVertexArrays(1, &m.opaqueVAO)
		m.opaqueVAO = 0
	}
	if m.translucentVBO != 0 {
		gl.DeleteBuffers(1, &m.translucentVBO)
		m.translucentVBO = 0
	}
	if m.translucentVAO != 0 {
		gl.DeleteVertexArrays(1, &m.translucentVAO)
		m.translucentVAO = 0
	}
	m.opaqueCount = 0
	m.translucentCount = 0
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

// evictLODMeshes drops sector meshes whose keys are not in `active`.
// Used to retire sectors that fell outside the horizon after a teleport
// or a long walk.
func (r *renderer) evictLODMeshes(active map[lodMeshKey]struct{}) {
	for k, m := range r.lodMeshes {
		if _, ok := active[k]; !ok {
			r.freeLODMesh(m)
			delete(r.lodMeshes, k)
		}
	}
}

// trimLODLRU evicts least-recently-used sector meshes until the cache
// is at or below `lruMeshBudget`. Called after each streaming pass so
// we don't accumulate unbounded distant geometry when the player walks
// a long way. Keys that appear in `pinned` are never evicted (used to
// protect sectors that are currently in the frustum).
func (r *renderer) trimLODLRU(pinned map[lodMeshKey]struct{}) {
	if len(r.lodMeshes) <= lruMeshBudget {
		return
	}
	type keyed struct {
		key   lodMeshKey
		stamp uint64
	}
	candidates := make([]keyed, 0, len(r.lodMeshes))
	for k, m := range r.lodMeshes {
		if _, pin := pinned[k]; pin {
			continue
		}
		candidates = append(candidates, keyed{k, m.lastUsed})
	}
	// Sort ascending by lastUsed stamp so the oldest entries evict first.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].stamp < candidates[j].stamp
	})
	excess := len(r.lodMeshes) - lruMeshBudget
	for i := 0; i < excess && i < len(candidates); i++ {
		k := candidates[i].key
		if m, ok := r.lodMeshes[k]; ok {
			r.freeLODMesh(m)
			delete(r.lodMeshes, k)
		}
	}
}

// freeLODMesh releases the GL resources owned by a distant-tier mesh.
func (r *renderer) freeLODMesh(m *lodMesh) {
	if m == nil {
		return
	}
	if m.opaqueVBO != 0 {
		gl.DeleteBuffers(1, &m.opaqueVBO)
		m.opaqueVBO = 0
	}
	if m.opaqueVAO != 0 {
		gl.DeleteVertexArrays(1, &m.opaqueVAO)
		m.opaqueVAO = 0
	}
	if m.translucentVBO != 0 {
		gl.DeleteBuffers(1, &m.translucentVBO)
		m.translucentVBO = 0
	}
	if m.translucentVAO != 0 {
		gl.DeleteVertexArrays(1, &m.translucentVAO)
		m.translucentVAO = 0
	}
	m.opaqueCount = 0
	m.translucentCount = 0
}

// uploadLODMesh replaces both vertex streams of a sector mesh.
func (r *renderer) uploadLODMesh(m *lodMesh, opaque, translucent []ChunkVertex) {
	m.opaqueCount = uploadVertexStream(&m.opaqueVAO, &m.opaqueVBO, opaque)
	m.translucentCount = uploadVertexStream(&m.translucentVAO, &m.translucentVBO, translucent)
}

// uploadMesh replaces both the opaque and translucent vertex streams for the
// given chunk. Each stream owns its own VAO+VBO; the attribute layout is
// identical so shader binding is shared between the two passes.
func (r *renderer) uploadMesh(m *chunkMesh, opaque, translucent []ChunkVertex) {
	m.opaqueCount = uploadVertexStream(&m.opaqueVAO, &m.opaqueVBO, opaque)
	m.translucentCount = uploadVertexStream(&m.translucentVAO, &m.translucentVBO, translucent)
	m.dirty = false
}

// uploadVertexStream lazily creates a VAO/VBO pair and uploads the given
// vertex slice to it. Returns the final vertex count (0 when verts is empty).
func uploadVertexStream(vao, vbo *uint32, verts []ChunkVertex) int32 {
	if *vao == 0 {
		gl.GenVertexArrays(1, vao)
		gl.GenBuffers(1, vbo)
		gl.BindVertexArray(*vao)
		gl.BindBuffer(gl.ARRAY_BUFFER, *vbo)

		gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, chunkVertexSize, 0)
		gl.EnableVertexAttribArray(0)
		gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, chunkVertexSize, 3*4)
		gl.EnableVertexAttribArray(1)
		gl.VertexAttribPointerWithOffset(2, 4, gl.FLOAT, false, chunkVertexSize, 6*4)
		gl.EnableVertexAttribArray(2)
		gl.VertexAttribPointerWithOffset(3, 4, gl.FLOAT, false, chunkVertexSize, 10*4)
		gl.EnableVertexAttribArray(3)
	} else {
		gl.BindVertexArray(*vao)
		gl.BindBuffer(gl.ARRAY_BUFFER, *vbo)
	}

	count := int32(len(verts))
	if count == 0 {
		// Orphan any existing data so we don't keep stale geometry alive.
		gl.BufferData(gl.ARRAY_BUFFER, 0, nil, gl.STATIC_DRAW)
		gl.BindBuffer(gl.ARRAY_BUFFER, 0)
		gl.BindVertexArray(0)
		return 0
	}

	gl.BufferData(gl.ARRAY_BUFFER, int(count)*int(chunkVertexSize), gl.Ptr(verts), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)
	return count
}
