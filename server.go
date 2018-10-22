package main

import (
	t "github.com/TyphoonMC/TyphoonCore"
	"strconv"
)

func runServer(game *Game) {
	core := t.Init()
	core.SetBrand("Crafty")

	core.On(func(e *t.PlayerJoinEvent) {
		msg := t.ChatMessage("Vous êtes connecté à un client Crafty")
		e.Player.SendMessage(msg)
	})

	core.DeclareCommand(t.CommandNodeLiteral("sb",
		[]*t.CommandNode{
			t.CommandNodeArgument("x",
				[]*t.CommandNode{
					t.CommandNodeArgument("y",
						[]*t.CommandNode{
							t.CommandNodeArgument("z",
								[]*t.CommandNode{
									t.CommandNodeArgument("type",
										nil,
										&t.CommandParserInteger{
											t.OptInteger{
												true,
												0,
											},
											t.OptInteger{
												true,
												int32(len(blocks) - 1),
											},
										},
										commandSetBlock(game),
									),
								},
								&t.CommandParserInteger{},
								nil,
							),
						},
						&t.CommandParserInteger{},
						nil,
					),
				},
				&t.CommandParserInteger{},
				nil,
			),
		},
		nil,
	))

	core.DeclareCommand(t.CommandNodeLiteral("tp",
		[]*t.CommandNode{
			t.CommandNodeArgument("x",
				[]*t.CommandNode{
					t.CommandNodeArgument("y",
						[]*t.CommandNode{
							t.CommandNodeArgument("z",
								nil,
								&t.CommandParserFloat{},
								commandTeleport(game),
							),
						},
						&t.CommandParserFloat{},
						nil,
					),
				},
				&t.CommandParserFloat{},
				nil,
			),
		},
		nil,
	))

	core.DeclareCommand(t.CommandNodeLiteral("gm",
		[]*t.CommandNode{
			t.CommandNodeLiteral("survival",
				[]*t.CommandNode{},
				func(player *t.Player, args []string) {
					game.player.gamemode = t.SURVIVAL
					player.SendMessage(t.ChatMessage("Gamemode: Survival"))
				},
			),
			t.CommandNodeLiteral("creative",
				[]*t.CommandNode{},
				func(player *t.Player, args []string) {
					game.player.gamemode = t.CREATIVE
					player.SendMessage(t.ChatMessage("Gamemode: Creative"))
				},
			),
			t.CommandNodeLiteral("spectator",
				[]*t.CommandNode{},
				func(player *t.Player, args []string) {
					game.player.gamemode = t.SPECTATOR
					player.SendMessage(t.ChatMessage("Gamemode: Spectator"))
				},
			),
		},
		nil,
	))

	core.DeclareCommand(t.CommandNodeLiteral("stop",
		nil,
		func(player *t.Player, args []string) {
			game.win.SetShouldClose(true)
		},
	))

	core.Start()
}

func commandSetBlock(game *Game) func(player *t.Player, args []string) {
	return func(player *t.Player, args []string) {
		x, _ := strconv.Atoi(args[1])
		y, _ := strconv.Atoi(args[2])
		z, _ := strconv.Atoi(args[3])
		id, _ := strconv.Atoi(args[4])
		game.setBlockAt(x, y, z, uint8(id))

		player.SendMessage(t.ChatMessage("Block placé"))
	}
}

func commandTeleport(game *Game) func(player *t.Player, args []string) {
	return func(player *t.Player, args []string) {
		x, _ := strconv.ParseFloat(args[1], 32)
		y, _ := strconv.ParseFloat(args[2], 32)
		z, _ := strconv.ParseFloat(args[3], 32)

		game.teleportPlayer(float32(x), float32(y), float32(z))

		player.SendMessage(t.ChatMessage("Téléporté"))
	}
}
