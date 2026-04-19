package game

import (
	"testing"
	"unsafe"
)

// TestChunkVertexSize is a compile-time contract: the chunk renderer
// hard-codes vertex attribute offsets that assume a 14-float layout. If this
// test fails, update renderer.go's gl.VertexAttribPointerWithOffset calls and
// the docstring on ChunkVertex before adjusting this expectation.
func TestChunkVertexSize(t *testing.T) {
	const expected = 14 * 4
	got := int(unsafe.Sizeof(ChunkVertex{}))
	if got != expected {
		t.Fatalf("ChunkVertex size = %d bytes, want %d", got, expected)
	}
}
