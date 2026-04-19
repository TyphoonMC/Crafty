# Contributing to Crafty

Thanks for wanting to contribute!

## Getting started

1. Fork and clone the repo (with submodules):
   ```
   git clone --recurse-submodules https://github.com/TyphoonMC/Crafty.git
   cd Crafty
   ```
2. Install **Go 1.22** or newer.
3. Install GLFW system dependencies:
   - **Linux (Debian/Ubuntu)**: `sudo apt-get install -y libgl1-mesa-dev xorg-dev`
   - **macOS**: Xcode Command Line Tools (`xcode-select --install`)
   - **Windows**: a MinGW-w64 toolchain (`tdm-gcc` or `msys2`)
4. Build and run:
   ```
   make run
   ```

## Development workflow

```
make fmt      # format
make vet      # go vet
make lint     # golangci-lint
make test     # unit tests with race detector
make security # govulncheck + gosec
```

Run all of them before pushing.

## Commit style

- Use imperative mood: "Add chunk serializer", not "Added..."
- Prefix with scope when useful: `game:`, `server:`, `ci:`, `docs:`
- Reference issues with `Fixes #123` when relevant

## Pull requests

- Keep PRs focused — one topic per PR
- Add tests when fixing bugs or adding logic
- Update `README.md` / `docs/` when changing user-visible behavior
- Make sure CI is green

## Reporting vulnerabilities

Please see [SECURITY.md](SECURITY.md). **Do not** open public issues for security problems.

## Code layout

The project follows the [Standard Go Project Layout](https://github.com/golang-standards/project-layout):

- `cmd/crafty/` — binary entry point
- `internal/game/` — game engine, rendering, world
- `internal/server/` — TyphoonCore-based admin server
- `configs/` — runtime configuration
- `build/` — packaging and CI helper files
- `scripts/` — dev/build scripts
- `docs/` — design and user documentation
