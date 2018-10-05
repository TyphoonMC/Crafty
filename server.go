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
												int32(len(blocks)-1),
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

	core.Start()
}

func commandSetBlock(game *Game) func(player *t.Player, args []string) {
	return func(player *t.Player, args []string) {
		x, err := strconv.Atoi(args[1])
		if err != nil {
			panic(err)
		}
		y, err := strconv.Atoi(args[2])
		if err != nil {
			panic(err)
		}
		z, err := strconv.Atoi(args[3])
		if err != nil {
			panic(err)
		}
		id, err := strconv.Atoi(args[4])
		if err != nil {
			panic(err)
		}
		game.setBlockAt(x, y, z, uint8(id))

		player.SendMessage(t.ChatMessage("Block placé "))
	}
}