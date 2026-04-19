# configs

Runtime configuration for Crafty.

- `config.json` — server defaults (listen address, compression, buffer limits).

Override at runtime by pointing the game at a different config path (future flag).
For now the binary expects this file at `./configs/config.json` relative to the
working directory.
