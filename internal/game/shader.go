package game

import (
	"fmt"
	"strings"

	"github.com/go-gl/gl/v3.3-core/gl"
)

// compileShader compiles a single GLSL stage. The source must end with a
// null byte — helper goStr takes care of it.
func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source + "\x00")
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))
		gl.DeleteShader(shader)
		return 0, fmt.Errorf("compile shader: %s", strings.TrimRight(log, "\x00"))
	}
	return shader, nil
}

// buildProgram compiles vertex + fragment GLSL sources into a linked program.
func buildProgram(vertSource, fragSource string) (uint32, error) {
	vert, err := compileShader(vertSource, gl.VERTEX_SHADER)
	if err != nil {
		return 0, fmt.Errorf("vertex: %w", err)
	}
	defer gl.DeleteShader(vert)

	frag, err := compileShader(fragSource, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, fmt.Errorf("fragment: %w", err)
	}
	defer gl.DeleteShader(frag)

	prog := gl.CreateProgram()
	gl.AttachShader(prog, vert)
	gl.AttachShader(prog, frag)
	gl.LinkProgram(prog)

	var status int32
	gl.GetProgramiv(prog, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(prog, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(prog, logLength, nil, gl.Str(log))
		gl.DeleteProgram(prog)
		return 0, fmt.Errorf("link program: %s", strings.TrimRight(log, "\x00"))
	}
	return prog, nil
}
