package main

type Player struct {
	pos FPoint3D
	rot FPoint3D
	speed float32
	cameraSpeed float32
}

func newPlayer() *Player {
	return &Player{
		FPoint3D{0, 3, 0},
		FPoint3D{0, 0, 0},
		0.2,
		0.2,
	}
}