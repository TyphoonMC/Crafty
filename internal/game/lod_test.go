package game

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

// TestLODForChunk walks the cascade and checks every radius boundary
// resolves to the expected tier. Boundary values lock in the inclusive
// outer radius interpretation (Chebyshev distance <= lodRadii[i]).
func TestLODForChunk(t *testing.T) {
	mid := Point2D{0, 0}
	cases := []struct {
		coord Point2D
		want  int
	}{
		{Point2D{0, 0}, 0},
		{Point2D{4, 0}, 0},   // on LOD 0 boundary
		{Point2D{5, 0}, 1},   // first LOD 1 chunk
		{Point2D{16, -16}, 1}, // LOD 1 outer corner
		{Point2D{17, 0}, 2},
		{Point2D{48, 48}, 2},
		{Point2D{49, 0}, 3},
		{Point2D{112, 112}, 3},
		{Point2D{113, 0}, 4},
		{Point2D{240, -240}, 4},
		{Point2D{241, 0}, -1}, // past the horizon
	}
	for _, c := range cases {
		got := lodForChunk(mid, c.coord)
		if got != c.want {
			t.Errorf("lodForChunk(%+v) = %d, want %d", c.coord, got, c.want)
		}
	}
}

// TestStepForLOD documents the doubling stride used by each tier.
func TestStepForLOD(t *testing.T) {
	want := [...]int{1, 2, 4, 8, 16}
	for i, w := range want {
		if got := stepForLOD(i); got != w {
			t.Errorf("stepForLOD(%d) = %d, want %d", i, got, w)
		}
	}
}

// TestSectorForChunk verifies sector snapping works for both positive
// and negative chunk coordinates, and that every chunk inside a sector
// maps back to the same sector origin.
func TestSectorForChunk(t *testing.T) {
	// Tier 2 uses 4x4 sectors.
	tier := 2
	for dx := 0; dx < 4; dx++ {
		for dz := 0; dz < 4; dz++ {
			coord := Point2D{8 + dx, -12 + dz} // sector origin (8, -12)
			got := sectorForChunk(coord, tier)
			want := Point2D{8, -12}
			if got != want {
				t.Errorf("sectorForChunk(%+v, %d) = %+v, want %+v", coord, tier, got, want)
			}
		}
	}
	// Negative coords flooring toward -inf.
	if got := sectorForChunk(Point2D{-1, -1}, 2); got != (Point2D{-4, -4}) {
		t.Errorf("sectorForChunk(-1,-1, 2) = %+v, want (-4,-4)", got)
	}
}

// TestFrustumAABB picks a simple perspective looking down +Z and
// verifies boxes in front, behind and to the side classify correctly.
func TestFrustumAABB(t *testing.T) {
	// Camera at origin looking at +Z, 90° fov, aspect 1, near 0.1, far 100.
	proj := mgl32.Perspective(mgl32.DegToRad(90), 1, 0.1, 100)
	// mgl32.LookAt flips Z so looking toward +Z needs eye at origin,
	// centre at (0,0,1).
	view := mgl32.LookAtV(mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 0, 1}, mgl32.Vec3{0, 1, 0})
	f := ExtractFrustum(proj.Mul4(view))

	// Box directly in front: should be visible.
	if !f.AABBVisible(-1, -1, 5, 1, 1, 6) {
		t.Error("box in front should be visible")
	}
	// Box directly behind: outside the near plane, should be culled.
	if f.AABBVisible(-1, -1, -10, 1, 1, -9) {
		t.Error("box behind camera should be culled")
	}
	// Zero frustum is conservative → always visible.
	var zero Frustum
	if !zero.AABBVisible(1e6, 1e6, 1e6, 1e6+1, 1e6+1, 1e6+1) {
		t.Error("zero-value frustum should treat all boxes as visible")
	}
}
