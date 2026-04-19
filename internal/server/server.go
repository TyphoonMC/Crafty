package server

import (
	"strconv"

	t "github.com/TyphoonMC/TyphoonCore"
	"github.com/TyphoonMC/Crafty/internal/game"
)

// Run starts the embedded TyphoonCore server exposing admin commands
// (/sb, /tp, /gm, /stop) that operate on the running game instance.
func Run(g *game.Game) {
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
											Min: t.OptInteger{Used: true, Value: 0},
											Max: t.OptInteger{Used: true, Value: int32(game.BlockCount() - 1)},
										},
										commandSetBlock(g),
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
								commandTeleport(g),
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
					g.SetGamemode(t.SURVIVAL)
					player.SendMessage(t.ChatMessage("Gamemode: Survival"))
				},
			),
			t.CommandNodeLiteral("creative",
				[]*t.CommandNode{},
				func(player *t.Player, args []string) {
					g.SetGamemode(t.CREATIVE)
					player.SendMessage(t.ChatMessage("Gamemode: Creative"))
				},
			),
			t.CommandNodeLiteral("spectator",
				[]*t.CommandNode{},
				func(player *t.Player, args []string) {
					g.SetGamemode(t.SPECTATOR)
					player.SendMessage(t.ChatMessage("Gamemode: Spectator"))
				},
			),
		},
		nil,
	))

	core.DeclareCommand(t.CommandNodeLiteral("stop",
		nil,
		func(player *t.Player, args []string) {
			g.Window().SetShouldClose(true)
		},
	))

	core.Start()
}

func commandSetBlock(g *game.Game) func(player *t.Player, args []string) {
	return func(player *t.Player, args []string) {
		x, _ := strconv.Atoi(args[1])
		y, _ := strconv.Atoi(args[2])
		z, _ := strconv.Atoi(args[3])
		id, _ := strconv.Atoi(args[4])
		g.SetBlockAt(x, y, z, uint8(id))

		player.SendMessage(t.ChatMessage("Block placé"))
	}
}

func commandTeleport(g *game.Game) func(player *t.Player, args []string) {
	return func(player *t.Player, args []string) {
		x, _ := strconv.ParseFloat(args[1], 32)
		y, _ := strconv.ParseFloat(args[2], 32)
		z, _ := strconv.ParseFloat(args[3], 32)

		g.TeleportPlayer(float32(x), float32(y), float32(z))

		player.SendMessage(t.ChatMessage("Téléporté"))
	}
}
