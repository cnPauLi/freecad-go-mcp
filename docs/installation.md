# Installation

[English](installation.md) · [中文](installation.zh.md) · [Back to README](../README.md) · [Configuration](configuration.md)

Install FreeCAD and the Go toolchain before setting up the addon and MCP client.
`go.mod` declares `go 1.26.2`; Go 1.21 and later download that toolchain
automatically when needed.

FreeCAD MCP has two components, and this repository ships both:

| Component | Location | Language |
| --- | --- | --- |
| FreeCAD addon (RPC server inside FreeCAD) | `addon/FreeCADMCP` | Python, runs inside FreeCAD |
| MCP server (launched by your MCP client) | repository root | Go |

Only the MCP server is built from this repository; the addon is a copy of the
[upstream addon](https://github.com/neka-nat/freecad-mcp) and needs no build step.

## Clone and build the MCP server

```bash
git clone https://github.com/cnPauLi/freecad-go-mcp.git
cd freecad-go-mcp
go build -o freecad-go-mcp .
```

The only dependency is [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go),
which `go build` fetches automatically. Behind a slow connection, use a mirror
first:

```bash
export GOPROXY=https://mirrors.aliyun.com/goproxy/,direct
```

> `go env -w GOPROXY=...` does not override a `GOPROXY` already present in your
> environment; export it on the command line as shown above.

You can also install the binary into `GOBIN` instead of the working directory:

```bash
go install github.com/cnPauLi/freecad-go-mcp@latest
```

## Install the addon

Copy the `addon/FreeCADMCP` directory from this repository into the addon
directory for your FreeCAD installation. The resulting directory should be
`Mod/FreeCADMCP`.

### Addon directory

| Platform / installation | Directory |
| --- | --- |
| Windows | `%APPDATA%\FreeCAD\Mod\` |
| macOS, FreeCAD 1.1 | `~/Library/Application Support/FreeCAD/v1-1/Mod/` |
| macOS, FreeCAD 1.0 | `~/Library/Application Support/FreeCAD/v1-0/Mod/` |
| Linux, Ubuntu | `~/.FreeCAD/Mod/` |
| Linux, Snap | `~/snap/freecad/common/Mod/` |
| Linux, Debian | `~/.local/share/FreeCAD/Mod/` |
| Linux, Arch / CachyOS (FreeCAD 1.1 from `extra/freecad`) | `~/.local/share/FreeCAD/v1-1/Mod/` |
| Linux, Flatpak | `~/.var/app/org.freecad.FreeCAD/data/FreeCAD/v1-1/Mod/` |

### Copy commands

Run the commands for your installation from the cloned repository.

Ubuntu:

```bash
mkdir -p ~/.FreeCAD/Mod/
cp -r addon/FreeCADMCP ~/.FreeCAD/Mod/
```

Debian:

```bash
mkdir -p ~/.local/share/FreeCAD/Mod/
cp -r addon/FreeCADMCP ~/.local/share/FreeCAD/Mod/
```

Arch / CachyOS, FreeCAD 1.1 from `extra/freecad`:

```bash
mkdir -p ~/.local/share/FreeCAD/v1-1/Mod/
cp -r addon/FreeCADMCP ~/.local/share/FreeCAD/v1-1/Mod/
```

Flatpak:

```bash
mkdir -p ~/.var/app/org.freecad.FreeCAD/data/FreeCAD/v1-1/Mod/
cp -r addon/FreeCADMCP ~/.var/app/org.freecad.FreeCAD/data/FreeCAD/v1-1/Mod/
```

macOS, FreeCAD 1.1:

```bash
mkdir -p ~/Library/Application\ Support/FreeCAD/v1-1/Mod/
cp -r addon/FreeCADMCP ~/Library/Application\ Support/FreeCAD/v1-1/Mod/
```

## Start the RPC server

Restart FreeCAD after installing the addon, then select **MCP Addon** from the
workbench list.

![MCP Addon in the workbench list](../assets/workbench_list.png)

Click **Start RPC Server** in the **FreeCAD MCP** toolbar.

![Start RPC Server toolbar button](../assets/start_rpc_server.png)

The server listens on `localhost:9875` and answers XML-RPC requests on `/RPC2`.
It starts manually by default. See
[auto-start configuration](configuration.md#auto-start-rpc-server) to enable it
on subsequent launches.

## Connect an MCP client

Point your client at the built binary. Replace `/path/to/freecad-go-mcp` with its
absolute path:

```json
{
  "mcpServers": {
    "freecad": {
      "command": "/path/to/freecad-go-mcp",
      "args": []
    }
  }
}
```

Restart the client after changing its configuration. Keep FreeCAD open with its
RPC server running. For remote FreeCAD installations, see
[remote connections](configuration.md#remote-connections).

### Run from source

For development, run the server straight from the checkout:

```bash
go run . --help
go test ./...
```

The MCP server writes its log to stderr and keeps stdout for the MCP protocol, so
a `go run` process works as a client `command` too:

```json
{
  "mcpServers": {
    "freecad": {
      "command": "go",
      "args": ["run", "/path/to/freecad-go-mcp"]
    }
  }
}
```

When changing addon code, copy the updated `addon/FreeCADMCP` directory into
FreeCAD's addon directory and restart FreeCAD as well.