package game

// Face direction indices used by both mesher.go (per-block meshing) and the
// chunk mesher. Keep in sync with BlockMesh.Faces.
const (
	FaceTop      = 0
	FaceBottom   = 1
	FaceForward  = 2
	FaceBackward = 3
	FaceLeft     = 4
	FaceRight    = 5
)

// faceOffsets maps each face index to its neighbour offset. Used by the
// chunk mesher to test whether a face is exposed.
var faceOffsets = [6]Point3D{
	{0, 1, 0},  // top
	{0, -1, 0}, // bottom
	{1, 0, 0},  // forward
	{-1, 0, 0}, // backward
	{0, 0, 1},  // left
	{0, 0, -1}, // right
}
