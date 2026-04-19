package game

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Frustum culling helpers. The 6 planes of the camera frustum are
// extracted from the combined proj*view matrix. Each plane is stored in
// Ax + By + Cz + D = 0 form with the normal pointing inward, so a point
// is inside the frustum iff every plane yields a non-negative value.
//
// A Frustum may also be the zero value, in which case `AABBVisible` is
// conservative and returns true. This lets streaming/rendering use the
// same code path on the first frame before the camera has any MVP yet.

type Frustum struct {
	// planes holds 6 entries (left, right, bottom, top, near, far).
	// Each plane is (A, B, C, D). Normals point inward; length(A,B,C)
	// is normalised to 1 so distances are in world units.
	planes [6][4]float32
	// valid is true when ExtractFrustum produced these planes. The
	// zero Frustum is invalid and treats everything as visible.
	valid bool
}

// ExtractFrustum decomposes the combined proj*view (or proj*view*model,
// but chunks render in world space so model = identity) into 6 inward
// planes. Reference: Gribb & Hartmann's "Fast Extraction of Viewing
// Frustum Planes from the World-View-Projection Matrix".
func ExtractFrustum(mvp mgl32.Mat4) Frustum {
	// mgl32 stores matrices column-major. mvp.At(row, col) picks the
	// element at (row, col); we want rows of the 4x4.
	row := func(r int) [4]float32 {
		return [4]float32{mvp.At(r, 0), mvp.At(r, 1), mvp.At(r, 2), mvp.At(r, 3)}
	}
	r0 := row(0)
	r1 := row(1)
	r2 := row(2)
	r3 := row(3)

	var f Frustum
	f.planes[0] = addRow(r3, r0)      // left:   row3 + row0
	f.planes[1] = subRow(r3, r0)      // right:  row3 - row0
	f.planes[2] = addRow(r3, r1)      // bottom: row3 + row1
	f.planes[3] = subRow(r3, r1)      // top:    row3 - row1
	f.planes[4] = addRow(r3, r2)      // near:   row3 + row2
	f.planes[5] = subRow(r3, r2)      // far:    row3 - row2
	for i := range f.planes {
		normalisePlane(&f.planes[i])
	}
	f.valid = true
	return f
}

func addRow(a, b [4]float32) [4]float32 {
	return [4]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2], a[3] + b[3]}
}

func subRow(a, b [4]float32) [4]float32 {
	return [4]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2], a[3] - b[3]}
}

func normalisePlane(p *[4]float32) {
	lx := p[0]
	ly := p[1]
	lz := p[2]
	l := lx*lx + ly*ly + lz*lz
	if l <= 1e-20 {
		return
	}
	inv := 1 / sqrt32(l)
	p[0] *= inv
	p[1] *= inv
	p[2] *= inv
	p[3] *= inv
}

func sqrt32(v float32) float32 {
	// math.Sqrt(float64) round-trip is fine here; the planes are only
	// normalised once per frame.
	return float32(math.Sqrt(float64(v)))
}

// AABBVisible reports whether the box [min, max] has any volume on the
// inside half-space of every plane (i.e. the box intersects or lies
// inside the frustum). Uses the p-vertex test: for each plane, pick the
// corner nearest the plane's normal and check it.
//
// A zero Frustum (never initialised) returns true so callers that skip
// culling on the first frame still render everything.
func (f Frustum) AABBVisible(minX, minY, minZ, maxX, maxY, maxZ float32) bool {
	if !f.valid {
		return true
	}
	for i := 0; i < 6; i++ {
		p := f.planes[i]
		// Pick the p-vertex: the corner of the box furthest in the
		// direction of the plane normal (i.e. maximising Ax+By+Cz).
		px := minX
		if p[0] >= 0 {
			px = maxX
		}
		py := minY
		if p[1] >= 0 {
			py = maxY
		}
		pz := minZ
		if p[2] >= 0 {
			pz = maxZ
		}
		if p[0]*px+p[1]*py+p[2]*pz+p[3] < 0 {
			// Box is fully outside this plane → fully outside the frustum.
			return false
		}
	}
	return true
}

// ChunkVisible is a convenience wrapper for a single chunk.
func (f Frustum) ChunkVisible(coord Point2D) bool {
	minX := float32(coord.x << 4)
	minZ := float32(coord.y << 4)
	return f.AABBVisible(minX, 0, minZ, minX+16, float32(worldHeight), minZ+16)
}

// SectorVisible tests the AABB of a LOD sector.
func (f Frustum) SectorVisible(sectorCoord Point2D, tier int) bool {
	minX, minY, minZ, maxX, maxY, maxZ := sectorAABB(sectorCoord, tier)
	return f.AABBVisible(minX, minY, minZ, maxX, maxY, maxZ)
}
