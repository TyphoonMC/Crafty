package main

import (
	"log"
	"runtime"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"

	"github.com/TyphoonMC/Crafty/internal/game"
	"github.com/TyphoonMC/Crafty/internal/server"
)

func main() {
	runtime.LockOSThread()

	log.Println("generating terrain...")
	g := game.NewGame()
	log.Println("starting...")

	if err := glfw.Init(); err != nil {
		log.Fatalln("failed to initialize glfw:", err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.Samples, 4)

	window, err := glfw.CreateWindow(1280, 800, "Crafty", nil, nil)
	if err != nil {
		log.Fatalln("failed to create window:", err)
	}
	g.SetWindow(window)
	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		log.Fatalln("failed to initialize gl:", err)
	}
	log.Printf("GL %s / GLSL %s", gl.GoStr(gl.GetString(gl.VERSION)), gl.GoStr(gl.GetString(gl.SHADING_LANGUAGE_VERSION)))

	if err := g.InitRenderer(); err != nil {
		log.Fatalln("failed to init renderer:", err)
	}
	g.InitInput()

	window.SetFramebufferSizeCallback(func(_ *glfw.Window, width, height int) {
		g.OnFramebufferResize(width, height)
	})
	// Prime the callback with the initial framebuffer size to handle Retina.
	fbW, fbH := window.GetFramebufferSize()
	g.OnFramebufferResize(fbW, fbH)

	go server.Run(g)

	for !window.ShouldClose() {
		g.MainLoop()
		window.SwapBuffers()
		glfw.PollEvents()
	}

	g.ShutdownRenderer()
	g.UnloadChunks()
}
