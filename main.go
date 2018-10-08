package main

import (
	"log"
	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	windowWidth  = 960
	windowHeight = 540
)

func main() {
	log.Println("generating terrain...")
	game := newGame()
	log.Println("starting...")

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
	game.win = window
	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		panic(err)
	}

	loadBlockTextures()

	game.initGl(window)
	game.initInput()

	go runServer(game)

	for !window.ShouldClose() {
		game.mainLoop()
		window.SwapBuffers()
		glfw.PollEvents()
	}

	unloadBlockTextures()
}

func (game *Game) initGl(win *glfw.Window) {
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

	ambient := []float32{2, 2, 2, 1}
	diffuse := []float32{1, 1, 1, 1}
	gl.Lightfv(gl.LIGHT0, gl.AMBIENT, &ambient[0])
	gl.Lightfv(gl.LIGHT0, gl.DIFFUSE, &diffuse[0])
	gl.Enable(gl.LIGHT0)

	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	ratio := float64(w)/float64(h)
	gl.Frustum(-1, 1, -1 * ratio, 1 * ratio, 1.0, 3000.0)
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
}

func (game *Game) camera() {
	h, w := game.win.GetSize()
	gl.Viewport(0, 0, int32(h), int32(w))

	gl.MatrixMode(gl.PROJECTION)
	mgl32.Perspective(60, float32(w)/float32(h), 1, 30000)
	/*ratio := float64(w)/float64(h)
	gl.Frustum(-1, 1, -1 * ratio, 1 * ratio, 1.0, 3000.0)*/

	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
	mgl32.LookAt(0, 60, -180,
		0, 50, 1,
		0, 0, 0)
}

func (game *Game) drawScene() {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()

	gl.Rotatef(game.player.rot.x, 1, 0, 0)
	gl.Rotatef(game.player.rot.y, 0, 1, 0)
	gl.Rotatef(game.player.rot.z, 0, 0, 1)
	gl.Translatef(game.player.pos.x, -game.player.pos.y, game.player.pos.z)

	for _, c := range game.chunks {
		coord := Point2D{c.coordinates.x*16, c.coordinates.y*16}
		for x, line := range c.blocks {
			for y, row := range line {
				for z, id := range row {
					game.drawBlock(x+coord.x, y, z+coord.y, id)
				}
			}
		}
	}
}

func (game *Game) mainLoop() {
	game.inputLoop()
	game.camera()
	game.drawScene()
}
