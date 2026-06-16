# specd Live Dashboard (VS Code)

A minimal VS Code extension that embeds the read-only `specd serve` dashboard in
a webview. It is a **separate package** from the specd Go binary and adds no
dependency to it.

## Usage

1. Build/install the `specd` binary and ensure it is on your `PATH` (or set
   `specd.binaryPath`).
2. Open a workspace containing a `.specd/` project.
3. Run the command **specd: Open Live Dashboard** (Command Palette) and pick a
   spec.

The extension launches `specd serve <slug>` on a loopback port
(`specd.servePort`, default 8765) and shows the dashboard in a panel. It is
strictly read-only: it only ever runs `specd serve`, which exposes no mutating
routes, and never writes spec state itself.

## Settings

| Setting | Default | Meaning |
|---------|---------|---------|
| `specd.binaryPath` | `specd` | Path to the specd binary. |
| `specd.servePort` | `8765` | Loopback port for the dashboard server. |

## Packaging

This is a plain JavaScript extension (no build step). To package for
distribution, install [`vsce`](https://github.com/microsoft/vscode-vsce) and run
`vsce package` in this directory.
