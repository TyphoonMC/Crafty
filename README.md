# Crafty

Tiny voxel game written in Go, using OpenGL 2.1 / GLFW and a
[TyphoonCore](https://github.com/TyphoonMC/TyphoonCore)-based admin server.

[![CI](https://github.com/TyphoonMC/Crafty/actions/workflows/ci.yml/badge.svg)](https://github.com/TyphoonMC/Crafty/actions/workflows/ci.yml)
[![Security](https://github.com/TyphoonMC/Crafty/actions/workflows/security.yml/badge.svg)](https://github.com/TyphoonMC/Crafty/actions/workflows/security.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/TyphoonMC/Crafty.svg)](https://pkg.go.dev/github.com/TyphoonMC/Crafty)
[![License](https://img.shields.io/github/license/TyphoonMC/Crafty)](LICENSE)

## Project layout

Follows the [Standard Go Project Layout](https://github.com/golang-standards/project-layout):

```
.
├── cmd/crafty/          # main entry point
├── internal/
│   ├── game/            # world, rendering, input, physics
│   └── server/          # TyphoonCore admin server
├── configs/             # runtime config
├── build/               # packaging, CI helpers
├── scripts/             # dev scripts
├── docs/                # design & user docs
├── Rosources/           # texture pack (git submodule)
└── .github/             # CI, security, templates, Dependabot
```

## Requirements

- **Go 1.22+**
- A C toolchain (CGo is required by `go-gl`)
- **Linux**: `sudo apt-get install -y libgl1-mesa-dev xorg-dev`
- **macOS**: `xcode-select --install`
- **Windows**: MinGW-w64 (`tdm-gcc` / `msys2`)

## Build

```sh
git clone --recurse-submodules https://github.com/TyphoonMC/Crafty.git
cd Crafty
make build
./bin/crafty
```

Or directly:

```sh
go build -o bin/crafty ./cmd/crafty
```

## Development

```sh
make fmt       # format
make vet       # go vet
make lint      # golangci-lint
make test      # race tests
make security  # govulncheck + gosec
make help      # list all targets
```

## Controls

| Key              | Action         |
|------------------|----------------|
| WASD / arrows    | Move           |
| Space            | Jump / fly up  |
| Left Shift       | Fly down       |
| Mouse left       | Break block    |
| Mouse right      | Place block    |
| Esc              | Release cursor |

## Admin commands (via TyphoonCore)

- `/sb <x> <y> <z> <type>` — set block
- `/tp <x> <y> <z>` — teleport
- `/gm <survival|creative|spectator>` — gamemode
- `/stop` — quit

## Security

Please report vulnerabilities privately — see [SECURITY.md](SECURITY.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

[GPL-3.0](LICENSE)
