package game

import (
	"fmt"
	"unsafe"

	"github.com/go-gl/gl/v3.3-core/gl"
)

// Overlay vertex layout: xy position in pixels (origin top-left), rgba colour
// as normalised floats. The vertex shader converts pixels to NDC using the
// current viewport size passed as a uniform.
type overlayVertex struct {
	x, y       float32
	r, g, b, a float32
}

const overlayVertexSize = int32(unsafe.Sizeof(overlayVertex{}))

const overlayVertexShaderSrc = `#version 330 core
layout (location = 0) in vec2 aPos;
layout (location = 1) in vec4 aColor;
uniform vec2 uScreenSize;
out vec4 vColor;
void main() {
    vec2 ndc = (aPos / uScreenSize) * 2.0 - 1.0;
    ndc.y = -ndc.y;
    gl_Position = vec4(ndc, 0.0, 1.0);
    vColor = aColor;
}
`

const overlayFragmentShaderSrc = `#version 330 core
in vec4 vColor;
out vec4 FragColor;
void main() { FragColor = vColor; }
`

// terminalRenderState owns the GPU resources for the overlay. It's lazily
// initialised on the first render call so New-less code paths (headless
// tests) don't require an OpenGL context.
type terminalRenderState struct {
	program     uint32
	uScreenSize int32
	vao         uint32
	vbo         uint32
	capacity    int // elements the VBO can currently hold
	verts       []overlayVertex
	initErr     error
}

func (t *Terminal) ensureRenderState() *terminalRenderState {
	if t.renderState == nil {
		t.renderState = &terminalRenderState{}
	}
	return t.renderState
}

func (s *terminalRenderState) init() error {
	if s.program != 0 || s.initErr != nil {
		return s.initErr
	}
	prog, err := buildProgram(overlayVertexShaderSrc, overlayFragmentShaderSrc)
	if err != nil {
		s.initErr = fmt.Errorf("build overlay program: %w", err)
		return s.initErr
	}
	s.program = prog
	s.uScreenSize = gl.GetUniformLocation(prog, gl.Str("uScreenSize\x00"))

	gl.GenVertexArrays(1, &s.vao)
	gl.GenBuffers(1, &s.vbo)
	gl.BindVertexArray(s.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, s.vbo)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, overlayVertexSize, 0)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(1, 4, gl.FLOAT, false, overlayVertexSize, 2*4)
	gl.EnableVertexAttribArray(1)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)
	return nil
}

// rect appends two triangles covering the rectangle [x, x+w) × [y, y+h) in
// the given colour.
func (s *terminalRenderState) rect(x, y, w, h float32, c [4]float32) {
	v := func(px, py float32) overlayVertex {
		return overlayVertex{px, py, c[0], c[1], c[2], c[3]}
	}
	s.verts = append(s.verts,
		v(x, y), v(x+w, y), v(x+w, y+h),
		v(x, y), v(x+w, y+h), v(x, y+h),
	)
}

// drawString rasterises text at (x0, y0) (top-left), with each bitmap pixel
// drawn as a scale×scale screen pixel quad. Returns the advance width in
// pixels.
func (s *terminalRenderState) drawString(text string, x0, y0, scale float32, color [4]float32) float32 {
	x := x0
	for _, r := range text {
		if r == '\n' {
			continue
		}
		rows := glyphRows(r)
		for row := 0; row < fontRows; row++ {
			bits := rows[row]
			for col := 0; col < fontCols; col++ {
				if bits&(1<<uint(col)) != 0 {
					px := x + float32(col)*scale
					py := y0 + float32(row)*scale
					s.rect(px, py, scale, scale, color)
				}
			}
		}
		x += float32(fontCols+fontKernPx) * scale
	}
	return x - x0
}

// textWidth returns the pixel width of text rendered at the given scale.
func textWidth(text string, scale float32) float32 {
	return float32(len([]rune(text))) * float32(fontCols+fontKernPx) * scale
}

// Render draws the terminal overlay. Call only while the GL context is
// current. screenW/H are framebuffer pixels. When the terminal is closed,
// no draw calls are issued (scrollback is only visible while open).
func (t *Terminal) Render(screenW, screenH int32) {
	if !t.open {
		return
	}
	if screenW <= 0 || screenH <= 0 {
		return
	}
	s := t.ensureRenderState()
	if err := s.init(); err != nil {
		return
	}

	s.verts = s.verts[:0]

	// Pastel palette consistent with Crafty branding.
	panelColor := [4]float32{0.15, 0.13, 0.20, 0.82}
	promptBg := [4]float32{0.20, 0.18, 0.26, 0.92}
	promptAccent := [4]float32{0.70, 0.56, 0.82, 1.0}
	textColor := [4]float32{0.95, 0.92, 0.88, 1.0}
	dimColor := [4]float32{0.70, 0.68, 0.78, 1.0}
	cursorColor := [4]float32{0.95, 0.92, 0.72, 1.0}

	w := float32(screenW)
	h := float32(screenH)

	// Choose a pixel scale that keeps the terminal readable on both small
	// windows and high-DPI framebuffers. One bitmap pixel = `scale` screen
	// pixels.
	scale := float32(2)
	if screenW >= 1600 {
		scale = 3
	}
	if screenW >= 2400 {
		scale = 4
	}

	lineH := float32(fontRows)*scale + scale*2 // glyph + leading
	panelH := lineH*10 + scale*4
	if panelH > h*0.45 {
		panelH = h * 0.45
	}
	panelY := h - panelH
	promptH := lineH + scale*2
	promptY := h - promptH

	// Panel background covers the output area.
	s.rect(0, panelY, w, panelH-promptH, panelColor)
	// Prompt strip at the very bottom with a slightly lighter tint so the
	// input line stands out.
	s.rect(0, promptY, w, promptH, promptBg)
	// Left-edge accent bar — a single pastel stripe the full height of the
	// overlay. Small touch of Crafty personality.
	s.rect(0, panelY, scale*2, panelH, promptAccent)

	// Output lines render bottom-up above the prompt. We lay out newest at
	// the bottom (just above the prompt strip) and stop when we run out of
	// space.
	maxOutputLines := int((panelH - promptH - scale*2) / lineH)
	if maxOutputLines < 0 {
		maxOutputLines = 0
	}
	// Expand each terminalLine into its newline-separated visual rows so
	// multi-line command output (like /help or /list) renders correctly.
	visualLines := make([]terminalLine, 0, len(t.output))
	for _, ln := range t.output {
		start := 0
		for i := 0; i < len(ln.text); i++ {
			if ln.text[i] == '\n' {
				visualLines = append(visualLines, terminalLine{text: ln.text[start:i], color: ln.color})
				start = i + 1
			}
		}
		visualLines = append(visualLines, terminalLine{text: ln.text[start:], color: ln.color})
	}
	startIdx := 0
	if len(visualLines) > maxOutputLines {
		startIdx = len(visualLines) - maxOutputLines
	}
	visible := visualLines[startIdx:]
	marginX := scale * 6
	outY := promptY - lineH
	for i := len(visible) - 1; i >= 0; i-- {
		ln := visible[i]
		col := [4]float32{
			float32(ln.color.R) / 255,
			float32(ln.color.G) / 255,
			float32(ln.color.B) / 255,
			1,
		}
		if ln.color.A == 0 {
			col = textColor
		}
		s.drawString(ln.text, marginX, outY, scale, col)
		outY -= lineH
		if outY < panelY {
			break
		}
	}

	// Prompt line: "> " + input + blinking caret. The caret is a filled
	// rectangle sized to one glyph cell. Draw the prompt prefix in an
	// accent tint, the text in the main ink colour.
	prefix := "> "
	px := marginX
	py := promptY + scale
	s.drawString(prefix, px, py, scale, promptAccent)
	px += textWidth(prefix, scale)

	inputStr := string(t.input)
	s.drawString(inputStr, px, py, scale, textColor)

	// Caret position: measure the width of the runes before the cursor.
	cursorStr := string(t.input[:t.cursor])
	caretX := px + textWidth(cursorStr, scale)
	// Always-on block caret; a slim vertical bar feels cleaner than the
	// classic blocky cursor and avoids needing a frame timer.
	s.rect(caretX, py, scale, float32(fontRows)*scale, cursorColor)

	// Hint line below the scrollback if there's space.
	if maxOutputLines > 0 {
		hint := "esc: close    enter: run    up/down: history"
		hintX := w - textWidth(hint, scale) - marginX
		s.drawString(hint, hintX, panelY+scale, scale, dimColor)
	}

	if len(s.verts) == 0 {
		return
	}

	// Commit GPU state for the overlay pass. Caller (drawScene) wraps this
	// in blend/depth toggles, but we re-assert blending here in case Render
	// is called from elsewhere.
	gl.UseProgram(s.program)
	gl.Uniform2f(s.uScreenSize, w, h)
	gl.BindVertexArray(s.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, s.vbo)
	byteSize := len(s.verts) * int(overlayVertexSize)
	if len(s.verts) > s.capacity {
		gl.BufferData(gl.ARRAY_BUFFER, byteSize, gl.Ptr(s.verts), gl.DYNAMIC_DRAW)
		s.capacity = len(s.verts)
	} else {
		gl.BufferSubData(gl.ARRAY_BUFFER, 0, byteSize, gl.Ptr(s.verts))
	}
	gl.DrawArrays(gl.TRIANGLES, 0, int32(len(s.verts)))
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)
}

// Shutdown releases GPU resources. Safe to call when Render was never run.
func (t *Terminal) Shutdown() {
	s := t.renderState
	if s == nil {
		return
	}
	if s.vbo != 0 {
		gl.DeleteBuffers(1, &s.vbo)
		s.vbo = 0
	}
	if s.vao != 0 {
		gl.DeleteVertexArrays(1, &s.vao)
		s.vao = 0
	}
	if s.program != 0 {
		gl.DeleteProgram(s.program)
		s.program = 0
	}
	t.renderState = nil
}
