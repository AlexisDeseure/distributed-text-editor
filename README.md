# Distributed, serverless text editor (Go + Fyne)

This project implements a lightweight, serverless collaborative text editor. Multiple machines connect directly over TCP to edit the same shared document—think "Google Docs" without a central server. It’s built in Go with a Fyne desktop UI.

## Architecture (3 layers)
- Application layer (Fyne GUI): local text editing UI, simple local log of edits, user actions.
- Control layer: distributed coordination (mutual exclusion) and routing between GUI and network.
- Network layer (TCP): peer connections and message transport; peers are discovered via target TCP addresses.

Each running instance is a "site" with a unique ID. Sites form an ad‑hoc overlay by connecting to one or more peers.

## Requirements
- Unix-like system (Linux/macOS). On Windows, use WSL (Windows Subsystem for Linux).
- Go 1.18+ installed inside that environment.
- A desktop environment to display the Fyne window (Wayland/X11 on Linux, macOS AppKit). On WSL, use WSLg (Windows 11) or an X server.

Tip: if scripts aren’t recognized after cloning on Windows, run `dos2unix` and make them executable.

```bash
chmod +x run.sh site.sh
# if needed
# dos2unix run.sh site.sh
```

## Quick start: multiple instances on one machine
Use `run.sh` to build binaries and launch N sites locally. It wires them randomly together and generates a network graph.

```bash
./run.sh --num-sites 4 --max-targets 2 --clean-output
```

What it does:
- Builds `build/network`, `build/controler`, `build/app`, `build/graph_generator`.
- Starts 4 GUI windows (one per site) on ports starting at 9000.
- Creates `output/network_topology.png` with the discovered topology.
- Stores runtime logs in `output/`.

Useful flags (from `run.sh`):
- `-n, --num-sites NUM` — number of sites to start (required if not prompted).
- `--base-port PORT` — first port to use (default: 9000).
- `--max-targets NUM` — max initial targets per site (default: 3).
- `--output-dir DIR` — output directory (default: `./output`).
- `--fifo-dir DIR` — FIFO dir (default: `/tmp`, internal wiring).
- `--clean-output` — clear outputs folder before start.

Stop with Ctrl+C; the script cleans up processes.

## Connect multiple machines over TCP
Launch a single site with `site.sh` and point it to peers using their TCP addresses. Repeat on each machine.

1) On Machine A (listening, no targets yet):
```bash
./site.sh --document "Shared doc" --port 9000
```
- Ensure inbound TCP on the chosen port (firewall/router as needed).
- Note A’s IP (e.g., 192.168.1.10).

2) On Machine B (connect to A):
```bash
./site.sh --document "Shared doc" --port 9001 --targets "192.168.1.10:9000"
```
- Multiple peers are allowed: `--targets "192.168.1.10:9000,192.168.1.20:9000"`.
- Additional machines can start and target any existing site; the overlay grows organically.

Common `site.sh` options:
- `--document NAME` — document shown in the UI (default includes a timestamp).
- `--port PORT` — TCP port to listen on (default: 9000).
- `--targets host:port[,host:port...]` — peers to connect to.
- `--output-dir DIR` — output directory (default: `./output`).
- `--already-built` — skip rebuild if binaries already exist.

Example with two peers:
```bash
./site.sh --document "Team notes" \
  --port 9100 \
  --targets "10.0.0.5:9000,10.0.0.6:9000"
```

Notes
- All machines must reach each other over TCP. Across NATs, use port‑forwarding or VPN.
- On Windows, run everything from a WSL shell (recommended: clone repo into the WSL filesystem).

## Outputs
- Logs: `output/*.log`
- Topology graph (via `run.sh`): `output/network_topology.png`

## Repository layout (short)
- `app/` — Fyne GUI and local document logic
- `controler/` — distributed control logic
- `network/` — TCP peer networking
- `graph_generator/` — renders the network graph image
- `build/` — compiled binaries (created by scripts)
- `output/` — logs and generated artifacts

That’s it — keep terminals open while editing; closing all sites ends the session.
