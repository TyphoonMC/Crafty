package game

import typhoon "github.com/TyphoonMC/TyphoonCore"

type Player struct {
	pos         FPoint3D
	rot         FPoint3D
	speed       float32
	cameraSpeed float32
	velocity    FPoint3D
	gamemode    typhoon.Gamemode

	// Physics state (survival mode only).
	onGround   bool
	wishDirX   float32
	wishDirZ   float32
	wantsJump  bool
}

func newPlayer() *Player {
	return &Player{
		pos:         FPoint3D{0, 110, 0},
		rot:         FPoint3D{0, 0, 0},
		speed:       0.2,
		cameraSpeed: 0.2,
		velocity:    FPoint3D{0, 0, 0},
		gamemode:    typhoon.SURVIVAL,
		onGround:    false,
		wishDirX:    0,
		wishDirZ:    0,
		wantsJump:   false,
	}
}
