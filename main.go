package main

import (
	"log"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"fmt"
)

const (
	windowWidth  = 960
	windowHeight = 540
)

func main() {
	fmt.Println("generating terrain...")
	game := newGame()
	fmt.Println("starting...")

	if err := glfw.Init(); err != nil {
		log.Fatalln("failed to initialize glfw:", err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	window, err := glfw.CreateWindow(800, 600, "Crafty", nil, nil)
	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		panic(err)
	}

	loadBlockTextures()

	initGl(window)

	go runServer(game)

	for !window.ShouldClose() {
		mainLoop(window, game)
		window.SwapBuffers()
		glfw.PollEvents()
	}

	unloadBlockTextures()
}

func initGl(win *glfw.Window) {
	h, w := win.GetSize()
	gl.Viewport(0, 0, int32(h), int32(w))
	gl.ShadeModel(gl.SMOOTH)
	gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.LIGHTING)
	gl.DepthFunc(gl.LESS)
	gl.Hint(gl.PERSPECTIVE_CORRECTION_HINT, gl.NICEST)
	gl.FrontFace(gl.CCW)
	gl.PolygonMode(gl.FRONT, gl.FILL)
	gl.PolygonMode(gl.BACK, gl.LINE)
	gl.CullFace(gl.BACK)
	gl.Disable(gl.CULL_FACE)

	gl.ClearColor(0.5, 0.5, 0.9, 0.0)
	gl.ClearDepth(1)
	gl.DepthFunc(gl.LEQUAL)

	ambient := []float32{0.5, 0.5, 0.5, 1}
	diffuse := []float32{1, 1, 1, 1}
	lightPosition := []float32{-5, 5, 10, 0}
	gl.Lightfv(gl.LIGHT0, gl.AMBIENT, &ambient[0])
	gl.Lightfv(gl.LIGHT0, gl.DIFFUSE, &diffuse[0])
	gl.Lightfv(gl.LIGHT0, gl.POSITION, &lightPosition[0])
	gl.Enable(gl.LIGHT0)

	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	gl.Frustum(-1, 1, -1, 1, 1.0, 3000.0)
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
}

func camera(win *glfw.Window) {
	h, w := win.GetSize()
	gl.Viewport(0, 0, int32(h), int32(w))

	gl.MatrixMode(gl.PROJECTION)
	mgl32.Perspective(60, float32(w)/float32(h), 1, 30000)

	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
	mgl32.LookAt(0, 60, -180,
		0, 50, 1,
		0, 1, 0)
}

func drawScene(game *Game) {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()

	gl.Translatef(0, -5, -20.0)

	for _, c := range game.chunks {
		coord := Point2D{c.coordinates.x*16, c.coordinates.y*16}
		for x, line := range c.blocks {
			for y, row := range line {
				for z, id := range row {
					drawBlock(game, x+coord.x, y, z+coord.y, id)
				}
			}
		}
	}
}

func mainLoop(win *glfw.Window, game *Game) {
	camera(win)
	drawScene(game)
}
