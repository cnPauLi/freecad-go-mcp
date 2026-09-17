# FreeCAD MCP (Go)

[English](README.md) · [中文](README.zh.md)

A Go reimplementation of the FreeCAD MCP client (MCP server), built on
[`github.com/mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) and talking to the
**FreeCAD addon** over XML-RPC. This is an independent project, hosted at
`github.com/cnPauLi/freecad-go-mcp`.

It is the Go rewrite of [neka-nat/freecad-mcp](https://github.com/neka-nat/freecad-mcp)
that **replaces only the MCP server**:

- The addon (`addon/FreeCADMCP`) comes from upstream and is redistributed here
  unchanged; it still runs inside FreeCAD exactly as before.
- Tool names, parameters, response text, and timeout policy match the upstream
  Python implementation, so MCP clients can switch over transparently.
- The MCP server is a standalone Go module (`github.com/cnPauLi/freecad-go-mcp`) that
  depends only on `mcp-go`. It needs no Python at runtime and does not depend on the
  upstream Python package.

## Origin and motivation

This project comes from the Python implementation of FreeCAD MCP in the parent repository
([neka-nat/freecad-mcp](https://github.com/neka-nat/freecad-mcp)): its MCP client, the
`src/freecad_mcp` package, is rewritten here in Go. The FreeCAD addon was left untouched,
so an addon that is already installed keeps working as it is. The binary version mirrors
the Python package version (`0.1.23`).

It was developed by an AI agent in a "vibe coding" workflow: the agent worked from the
Python sources and their tests, reproduced the observable behaviour, and then verified the
result end to end against a running FreeCAD instance. See
[parity with the Python implementation](#parity-with-the-python-implementation) for what
is matched deliberately.

The motivation is deployment simplicity:

- no Python runtime, no `uv`/`uvx`, no virtual environment, and no dependency resolution
  on the client side;
- one statically linked binary with no external library dependencies, which an MCP client
  can launch directly;
- the same tools, parameters, and response text, so nothing has to change inside FreeCAD.

## Demo

### Designing a flange

![Designing a flange in FreeCAD](assets/freecad_mcp4.gif)

### Designing a toy car

![Designing a toy car in FreeCAD](assets/make_toycar4.gif)

### Designing a part from a 2D drawing

Input drawing:

![Input 2D drawing](assets/b9-1.png)

Modelling:

![Modelling the part from the drawing](assets/from_2ddrawing.gif)

More demos and example scripts are in the [examples doc](docs/examples.md).

## Quick start

You need FreeCAD (with the addon from this repository installed) and the Go
toolchain (`go.mod` declares `go 1.26.2`). The layout matches the Python version:
an addon running inside FreeCAD, and an MCP server launched by your MCP client.

### 1. Install and start the FreeCAD addon

Copy `addon/FreeCADMCP` into your
[FreeCAD addon directory](docs/installation.md#addon-directory) and restart FreeCAD.
Switch to the **MCP Addon** workbench and click **Start RPC Server** in the
**FreeCAD MCP** toolbar. The RPC server listens on `localhost:9875` and answers on
`/RPC2`.

Per-platform commands and screenshots are in the [installation guide](docs/installation.md).

### 2. Build the MCP server

```bash
git clone https://github.com/cnPauLi/freecad-go-mcp.git
cd freecad-go-mcp
go build -o freecad-go-mcp .
```

The only dependency is `mcp-go v1.1.0`, fetched automatically by `go build`. Behind a
slow connection, use the Aliyun mirror first:

```bash
export GOPROXY=https://mirrors.aliyun.com/goproxy/,direct
```

> `go env -w GOPROXY=...` does not override a `GOPROXY` already present in your
> environment; export it on the command line as shown above.

The Makefile wraps this and produces a static binary:

```bash
make build   # -> ./freecad-go-mcp
```

### 3. Configure your MCP client

Add an entry to your client's MCP configuration, with `command` set to the absolute
path of the binary:

```json
{
  "mcpServers": {
    "freecad": {
      "command": "/absolute/path/to/freecad-go-mcp",
      "args": []
    }
  }
}
```

Restart the client to load the configuration, keep FreeCAD open with its RPC server
running, and the model can start modelling. Connections use `localhost` by default,
and logs go to **stderr** (stdout is reserved for the MCP protocol itself).

## Configuration

### Command-line flags

| Flag | Default | Description |
| --- | --- | --- |
| `--only-text-feedback` | off | Omit optional screenshots to reduce token use. |
| `--host` | `localhost` | Host running the FreeCAD RPC server. Validated on startup; must be a valid IPv4/IPv6 address or hostname, otherwise the server exits with code 2. |
| `--freecadcmd` | auto-detected | Command used to launch headless FreeCAD, for example `flatpak run --command=freecadcmd org.freecad.FreeCAD`. Parsed with shell word-splitting rules, so quotes and escapes work; an unterminated quote fails at startup. |

### Text feedback and screenshots

```json
{
  "mcpServers": {
    "freecad": {
      "command": "/absolute/path/to/freecad-go-mcp",
      "args": ["--only-text-feedback"]
    }
  }
}
```

Put only the flags you want to pass through in `args`, for example
`["--only-text-feedback"]` or `["--host", "192.168.1.100"]`.

Optional screenshots can also be controlled per call with `include_screenshot` and
`view_name`. The global `--only-text-feedback` flag takes precedence over
`include_screenshot`. See [screenshot options](#screenshot-options) for the applicable
tools and the explicit `get_view` tool.

### Remote connections

By default the server only listens on `localhost` and rejects remote connections. To
control FreeCAD from another machine, configure both the addon and the MCP client:

1. Check **Remote Connections** in the **FreeCAD MCP** toolbar. On the next server
   restart it binds `0.0.0.0`; only addresses in **Allowed IPs** (default `127.0.0.1`)
   or their CIDR subnets can connect.
2. Click **Configure Allowed IPs** and enter a comma-separated allow list, for example
   `192.168.1.100, 10.0.0.0/24`. Invalid entries are rejected.
3. Restart the RPC server after changing the settings.
4. Pass `--host` on the MCP client side:

```json
{
  "mcpServers": {
    "freecad": {
      "command": "/absolute/path/to/freecad-go-mcp",
      "args": ["--host", "192.168.1.100"]
    }
  }
}
```

`--host` selects the **GUI RPC** host only; [headless execution](#headless-execution)
runs on the machine hosting the MCP server, so its file paths must be accessible there.

## Tools

17 tools, identical to the Python implementation.

| Tool | Purpose |
| --- | --- |
| `create_document` | Create a new FreeCAD document. |
| `list_documents` | List open documents. |
| `reload_document` | Close and reopen a saved document to pick up external file changes, such as results from a headless script. |
| `create_object` | Create an object in a document. |
| `edit_object` | Edit an object's properties. |
| `delete_object` | Delete an object from a document. |
| `get_objects` | Get all objects in a document. |
| `get_object` | Get one object in a document. |
| `get_view` | Get a screenshot of the active view. |
| `execute_code` | Execute Python code on FreeCAD's GUI thread. |
| `execute_code_async` | Start a background computation and return its job ID; use `commit()` for document and view access. |
| `get_async_status` | Get background job state and failure tracebacks without using the GUI thread. |
| `execute_code_headless` | Run a script in a separate `freecadcmd` process and return its exit status and output. |
| `get_rpc_status` | Report RPC and GUI-dispatch health without using the GUI thread. |
| `insert_part_from_library` | Insert a part from the [FreeCAD parts library](https://github.com/FreeCAD/FreeCAD-library). |
| `get_parts_list` | List parts in the [FreeCAD parts library](https://github.com/FreeCAD/FreeCAD-library). |
| `run_fem_analysis` | Run CalculiX on an existing analysis and return summary results. |

Besides the tools, one prompt is registered: `asset_creation_strategy`, with the same
content as the Python implementation.

## Screenshot options

The following tools return optional screenshots: `create_object`, `edit_object`,
`delete_object`, `execute_code`, `insert_part_from_library`, `get_objects`,
`get_object`, and `run_fem_analysis`.

| Parameter | Default | Purpose |
| --- | --- | --- |
| `include_screenshot` | `true` | Set to `false` for text-only feedback, such as analytical scripts or intermediate steps. |
| `view_name` | `"Isometric"` | Orient the returned screenshot, for example `"Front"`, `"Top"`, or `"Right"`. |

[`--only-text-feedback`](#text-feedback-and-screenshots) suppresses these optional
screenshots regardless of `include_screenshot`.

Use `get_view` to request a screenshot explicitly; it is available even with
`--only-text-feedback`. It takes `view_name` and optional `width`, `height`, and
`focus_object` parameters. Supported views are `Isometric`, `Front`, `Top`, `Right`,
`Back`, `Left`, `Bottom`, `Dimetric`, and `Trimetric`.

## FEM analysis

`run_fem_analysis` runs the CalculiX solver on an existing `Fem::FemAnalysis`
container. It auto-creates a `SolverCcxTools` if the analysis has none and returns max
von Mises stress, max displacement, node count, and the solver's working directory.
The default `timeout` is 600 seconds. For long analyses, let the client allow the
[queue and execution timeout budgets](#gui-dispatch-timeouts).

For an end-to-end example (geometry, material, mesh, constraints, and a comparison with
an analytical solution), see
[`examples/cantilever_fem.py`](https://github.com/neka-nat/freecad-mcp/blob/main/examples/cantilever_fem.py)
in the upstream repository.

## Code execution

### Three execution modes

| Tool | Use case |
| --- | --- |
| `execute_code` | Normal FreeCAD automation; the script runs on the GUI thread. |
| `execute_code_async` | Long background computations on independent geometry; hand document and view access to the GUI thread with `commit()`. |
| `execute_code_headless` | Heavy OCCT operations in a separate process, isolating native crashes from the GUI. |

### Shared script namespace

`execute_code` and `execute_code_async` share a persistent script namespace with
`FreeCAD`/`App` and `FreeCADGui`/`Gui` aliases. Variables survive between calls without
overwriting the RPC server's own functions. Concurrent scripts share the same live
variables and must coordinate writes to the same data. Headless scripts run in a fresh
process and do not share this namespace.

### Background jobs

`execute_code_async` returns a `job_id`. `get_async_status(job_id)` reports whether the
job is `running`, `done`, or `failed`, and includes the exception and traceback for
failed jobs.

All running jobs and the 20 most recently completed jobs are kept in memory until
FreeCAD exits. `get_async_status()` lists this history; `get_rpc_status` lists the IDs
of jobs still running. Job status does not wait for GUI cleanup, and a successful script
does not certify that the geometry is valid.

Job status requires the updated addon. With an older addon, keep polling a document
status object and checking FreeCAD's Report View.

#### Document and view access

Async code must keep document and view access on the GUI thread: build independent OCCT
shapes in the worker, then use `commit(fn, timeout=120)` to apply the result and
recompute the document on the GUI thread. The helper returns `fn`'s value, or raises
`RuntimeError` on failure. `commit()` persists in the shared namespace so later async
calls can reuse it; calling it from `execute_code` or inside a GUI callback fails
immediately.

### Headless execution

`execute_code_headless` writes the script to a file and runs it with `freecadcmd -c` in
a separate process. Use it for OpenCascade work that may segfault or block the GUI for
minutes: `makeHelix` + `makePipeShell` threads, lofts and sweeps, or booleans with many
B-spline tools. A native crash only ends the helper process; the tool reports the signal
(e.g. `SIGSEGV`) together with everything the script printed, and the GUI keeps its
documents.

The script must import the modules it needs and open and save documents itself
(`FreeCAD.openDocument`, `doc.save()`, `doc.saveAs()`, or `Shape.exportBrep`). After
saving an `.FCStd` file that is open in the GUI, use `reload_document(doc_name)` to
refresh the GUI copy; get the document name with `list_documents`.

The executable runs on the machine hosting the MCP server, and `--host` only selects the
GUI RPC host, so use paths accessible on the MCP server machine. `timeout` must be
positive and finite (default: 600 seconds); a timeout returns partial stdout/stderr, and
temporary scripts are removed on success, failure, and timeout (temporary scripts live in
`~/.cache/freecad-mcp/headless`, because Flatpak sandboxes usually cannot see `/tmp`).

The executable is auto-detected in order: `freecadcmd`, `FreeCADCmd`, `freecadcmd.exe`,
and `freecad.cmd` on `PATH`, then the `org.freecad.FreeCAD` Flatpak. Override it with:

```json
{
  "mcpServers": {
    "freecad": {
      "command": "/absolute/path/to/freecad-go-mcp",
      "args": ["--freecadcmd", "flatpak run --command=freecadcmd org.freecad.FreeCAD"]
    }
  }
}
```

### GUI dispatch timeouts

GUI-thread operations run one at a time in FIFO order. Calls have separate queue and
execution budgets: execution time counts from the moment a call starts on the GUI thread,
and waiting for an earlier operation does not consume that budget. The queue budget
defaults to the execution budget; if it expires before a call starts, the call is
cancelled and will not run later, and this does not mark dispatch as stuck.

| Operation | Queue budget | Execution budget | Go client socket timeout |
| --- | --- | --- | --- |
| `execute_code` | 90 seconds | 90 seconds | At least 210 seconds |
| `run_fem_analysis` | Requested `timeout` | Requested `timeout` | At least `2 * timeout + 30` seconds |

The client timeout covers both budgets plus a 30-second margin, with a 150-second base
value: it is the larger of 150 seconds and the per-operation budget above. Other MCP
clients and hosts must allow these response times too. Concurrent `execute_code` calls
are queued but still run sequentially, so total wall time is the sum of the individual
runs.

### Recover from a stuck GUI operation

If a GUI-thread operation exceeds its execution budget after starting, the bridge returns
`GUI_DISPATCH_STUCK` and rejects later GUI operations immediately. Calls that were
already queued keep waiting, up to their queue timeout, and run once the stuck operation
returns.

Use `get_rpc_status` to identify the operation that is still running without queuing: the
RPC server handles connections concurrently and each Go call uses its own HTTP
connection, so `get_rpc_status` and `get_async_status` still return immediately while
`execute_code` is blocked. Document queries (`get_object`, `get_objects`,
`list_documents`) run on the GUI thread like modelling operations and return an RPC fault
when dispatch times out or is stuck.

FreeCAD GUI work cannot be force-cancelled safely. If status does not return to `healthy`
after the operation finishes, restart FreeCAD.

After an `execute_code` exception on a FreeCAD development build, inspect any new
`FeaturePython` object before mutating or deleting it. In particular, do not continue
with an object whose required `Proxy` was never installed, as touching that broken object
can wedge FreeCAD's GUI thread.

## Project layout

```
freecad-go-mcp/
├── addon/FreeCADMCP/           # FreeCAD addon (upstream, redistributed as-is, not part of the Go build)
├── assets/                     # Screenshots and demo GIFs used by the docs
├── docs/                       # Installation, configuration, tools, execution, examples (English + .zh Chinese)
├── .github/workflows/test.yml  # CI: gofmt / go vet / go build / go test
├── main.go                     # CLI validation, state setup, MCP stdio startup
├── main_test.go                # --host validation cases
└── internal/
    ├── xmlrpc/                 # XML-RPC encoding/decoding (<nil/>, struct, array, int/double, fault, base64)
    ├── freecad/
    │   ├── client.go           # RPC client: port 9875 /RPC2, timeout budgets, result conversion
    │   └── headless.go         # freecadcmd detection, shell word-splitting, crash/timeout handling, output cleanup
    └── tools/
        ├── tools.go            # Tool and prompt registration, argument reading (missing vs. empty)
        ├── operations.go       # Behaviour and text for the 17 tools
        ├── descriptions.go     # Tool descriptions
        ├── prompt.go           # asset_creation_strategy prompt
        ├── responses.go        # Text / JSON / screenshot result helpers
        └── state.go            # Lazily initialised config and persistent connection
```

Each package keeps its `*_test.go` files next to the implementation.

## Parity with the Python implementation

The rewrite matches these observable behaviours:

- **Tool set and text**: 17 tools plus 1 prompt, with success/failure messages matching
  Python's `operations/core.py` verbatim; JSON output uses `indent=2`, does not escape
  HTML, and preserves UTF-8 (Go prints map keys in lexicographic order while Python uses
  insertion order — the only intentional difference).
- **Timeout budgets**: see [GUI dispatch timeouts](#gui-dispatch-timeouts); they match the
  assertions in `tests/test_client_timeouts.py`.
- **Connections and concurrency**: the connection is created lazily on first use and
  pinged; on failure the tool returns
  `Failed to connect to FreeCAD. Make sure the FreeCAD addon is running.`
  as an `isError=true` tool result (not an MCP protocol-level error), matching Python's
  FastMCP behaviour.
- **Screenshot failures are non-fatal**: a failed screenshot is only logged and treated as
  "no screenshot"; it does not affect the main tool result.
- **No Python runtime dependency**: the MCP server needs no Python; only
  `execute_code_headless` invokes `freecadcmd` (or whatever `--freecadcmd` specifies).

## Cross-compiling and releases

`make dist` cross-compiles static binaries into `dist/`. Builds run with `CGO_ENABLED=0`,
so they link no external library:

| OS | Architectures |
| --- | --- |
| Linux | amd64 (x86_64), arm64 |
| macOS | amd64 (Intel), arm64 (Apple silicon) |
| Windows | amd64 (x86_64), arm64 |

```bash
make dist       # dist/freecad-go-mcp_<version>_<os>_<arch>[.exe], six binaries
make checksums  # additionally writes dist/SHA256SUMS
make check      # gofmt + go vet + go test
make clean      # remove dist/ and the host binary
```

`dist/` is git-ignored. The version comes from `git describe --tags` and is embedded with
`-ldflags`, so tag the commit before building a release.

Pushing a `v*` tag runs [.github/workflows/release.yml](.github/workflows/release.yml): it
runs `make check`, cross-compiles all six binaries, verifies the Linux ones are statically
linked, and publishes them together with `SHA256SUMS` as a GitHub release. Starting the
same workflow manually from the Actions tab uploads the binaries as a build artifact
instead of creating a release.

## Tests

```bash
git clone https://github.com/cnPauLi/freecad-go-mcp.git
cd freecad-go-mcp
go test ./...
```

The tests spin up a fake XML-RPC server with `httptest` and cover: XML-RPC
encoding/decoding (including faults and pointer/named types), timeout budgets,
concurrent availability while a call is blocked, the headless success/failure/crash/
timeout paths (running real scripts through `python3` instead of `freecadcmd`), per-tool
text and screenshot policy, the tool registry, and CLI argument validation. Export
`GOPROXY` first if dependency downloads are slow.

## Documentation

| Document | Contents |
| --- | --- |
| [Installation](docs/installation.md) | Addon directories, screenshots, build and client setup |
| [Configuration](docs/configuration.md) | Auto-start, text feedback, remote connections |
| [Tools](docs/tools.md) | Tool list, screenshots, FEM analysis |
| [Code execution](docs/execution.md) | GUI execution, background jobs, headless scripts, timeout recovery |
| [Examples](docs/examples.md) | Design demos, FEM example, ADK and LangChain integrations |
| [Upstream project](https://github.com/neka-nat/freecad-mcp) | Original Python implementation and addon source |