# FreeCAD MCP（Go 版）

[English](README.md) · [中文](README.zh.md)

用 Go 重新实现的 FreeCAD MCP 客户端（MCP server），以
[`github.com/mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) 为 MCP 开发基础，
通过 XML-RPC 对接 **FreeCAD 插件（addon）** 的 RPC 服务。这是一个独立项目，仓库地址为
`github.com/cnPauLi/freecad-go-mcp`。

本仓库对应 [neka-nat/freecad-mcp](https://github.com/neka-nat/freecad-mcp) 的 Go 重写版，
**只替换 MCP 服务端**：

- 插件（`addon/FreeCADMCP`）来自上游并原样随本仓库分发，**不做任何改动**，
  仍按原方式安装在 FreeCAD 中运行；
- 工具名称、参数、返回文案、超时策略都与上游 Python 版保持一致，MCP 客户端可以无感替换；
- MCP server 是一个独立的 Go module（`github.com/cnPauLi/freecad-go-mcp`），
  只依赖 `mcp-go`，运行时不依赖 Python，也不依赖上游的 Python 包。

## 项目来源与动机

本项目来源于上层目录仓库中的 Python 版 FreeCAD MCP
（[neka-nat/freecad-mcp](https://github.com/neka-nat/freecad-mcp)）：这里把它的 MCP 客户端
（`src/freecad_mcp` 包）用 Go 重写。FreeCAD 插件（addon）保持原样未做改动，因此已经安装好的
插件可以继续使用。二进制的版本号与 Python 包版本保持一致（`0.1.23`）。

开发过程采用 AI agent 的 "vibe coding" 方式：由 agent 依据 Python 源码及其测试复刻可观察行为，
再对着真实运行的 FreeCAD 做端到端验证。刻意对齐的部分见
[与 Python 实现的一致性](#与-python-实现的一致性)。

动机是部署简单：

- 客户端侧不再需要 Python 运行时、`uv`/`uvx`、虚拟环境，也不需要解析依赖；
- 单个静态链接的二进制（不依赖任何外部库），MCP 客户端可以直接拉起；
- 工具、参数与返回文案完全一致，FreeCAD 一侧无需任何改动。

## 演示

### 设计法兰

![在 FreeCAD 中设计法兰](assets/freecad_mcp4.gif)

### 设计玩具车

![在 FreeCAD 中设计玩具车](assets/make_toycar4.gif)

### 从 2D 图纸建模

输入图纸：

![输入 2D 图纸](assets/b9-1.png)

建模过程：

![依据图纸建模](assets/from_2ddrawing.gif)

更多演示与示例脚本见[示例文档](docs/examples.zh.md)。

## 快速开始

需要 FreeCAD（已安装本仓库的 addon）和 Go 工具链（`go.mod` 声明 `go 1.26.2`）。
整体结构与 Python 版相同：一个跑在 FreeCAD 里的 addon，一个由 MCP 客户端拉起的 MCP server。

### 1. 安装并启动 FreeCAD addon

把 `addon/FreeCADMCP` 复制到你的 [FreeCAD 插件目录](docs/installation.zh.md#插件目录)
并重启 FreeCAD。切换到 **MCP Addon** 工作台，在 **FreeCAD MCP** 工具栏点击
**Start RPC Server**。RPC 服务默认监听 `localhost:9875`，请求路径为 `/RPC2`。

各平台的安装命令与截图见[安装指南](docs/installation.zh.md)。

### 2. 构建 MCP server

```bash
git clone https://github.com/cnPauLi/freecad-go-mcp.git
cd freecad-go-mcp
go build -o freecad-go-mcp .
```

依赖只有 `mcp-go v1.1.0`，`go build` 会自动拉取。国内网络建议使用阿里云代理加速：

```bash
export GOPROXY=https://mirrors.aliyun.com/goproxy/,direct
```

> 注意：若 shell 环境里已存在 `GOPROXY` 变量，`go env -w GOPROXY=...` 会被环境变量覆盖并给出警告，
> 此时需要在命令前 `export`（如上）。

Makefile 封装了同样的构建过程，产出静态二进制：

```bash
make build   # -> ./freecad-go-mcp
```

### 3. 配置 MCP 客户端

在客户端的 MCP 配置中加入一项（`command` 填二进制的绝对路径）：

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

重启客户端加载配置，保持 FreeCAD 打开且 RPC server 处于运行状态，即可让模型开始建模。
连接默认使用 `localhost`，日志写到 **stderr**（stdout 保留给 MCP 协议本身）。

## 配置

### 命令行参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `--only-text-feedback` | 关闭 | 不返回可选的截图，减少 token 消耗。 |
| `--host` | `localhost` | FreeCAD RPC server 所在主机。启动时校验，必须是合法的 IPv4/IPv6 地址或主机名，否则报错退出（退出码 2）。 |
| `--freecadcmd` | 自动探测 | 启动 headless FreeCAD 的命令，例如 `flatpak run --command=freecadcmd org.freecad.FreeCAD`。按 shell 词法解析，支持引号与转义；引号未闭合等错误会在启动时报错退出。 |

### 纯文本反馈与截图

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

`args` 只填需要透传的参数，例如 `["--only-text-feedback"]`、`["--host", "192.168.1.100"]`。

也可以在单次调用中用 `include_screenshot` 和 `view_name` 控制可选截图，
**全局 `--only-text-feedback` 优先级高于 `include_screenshot`**。
适用工具与显式的 `get_view` 工具见[截图选项](#截图选项)。

### 远程连接

默认只监听 `localhost`、不接受远程连接。要从另一台机器控制 FreeCAD，需要同时配置 addon 与 MCP 客户端：

1. 在 FreeCAD 的 **FreeCAD MCP** 工具栏勾选 **Remote Connections**，
   下次重启 server 时绑定 `0.0.0.0`；只有 **Allowed IPs** 中的 IP 或 CIDR 子网（默认 `127.0.0.1`）可以连接。
2. 点击 **Configure Allowed IPs** 填入逗号分隔的白名单，例如 `192.168.1.100, 10.0.0.0/24`，非法条目会被拒绝。
3. 修改后重启 RPC server。
4. MCP 客户端侧传入 `--host`：

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

`--host` 只选择 **GUI RPC** 主机；[headless 执行](#headless-执行)跑在 MCP server 所在的机器上，
因此文件路径必须在 MCP server 那台机器上可访问。

## 工具列表

共 17 个工具，与 Python 版完全一致。

| 工具 | 用途 |
| --- | --- |
| `create_document` | 新建 FreeCAD 文档。 |
| `list_documents` | 列出已打开的文档。 |
| `reload_document` | 关闭并重新打开已保存的文档，以获取外部文件改动（例如 headless 脚本产出的结果）。 |
| `create_object` | 在文档中创建对象。 |
| `edit_object` | 修改对象属性。 |
| `delete_object` | 从文档中删除对象。 |
| `get_objects` | 获取文档中的全部对象。 |
| `get_object` | 获取文档中的单个对象。 |
| `get_view` | 获取当前视图的截图。 |
| `execute_code` | 在 FreeCAD 的 GUI 线程上执行 Python 代码。 |
| `execute_code_async` | 启动后台计算并返回 job ID；读写文档与视图须使用 `commit()`。 |
| `get_async_status` | 不占用 GUI 线程即可查询后台任务状态与失败堆栈。 |
| `execute_code_headless` | 在独立的 `freecadcmd` 进程中运行脚本，返回退出状态与输出。 |
| `get_rpc_status` | 不占用 GUI 线程即可查询 RPC 与 GUI 调度健康状态。 |
| `insert_part_from_library` | 从 [FreeCAD 零件库](https://github.com/FreeCAD/FreeCAD-library)插入零件。 |
| `get_parts_list` | 列出 [FreeCAD 零件库](https://github.com/FreeCAD/FreeCAD-library)中的零件。 |
| `run_fem_analysis` | 对已有分析容器运行 CalculiX 并返回汇总结果。 |

除工具外还提供一个 prompt：`asset_creation_strategy`（资产创建策略），与 Python 版内容一致。

## 截图选项

以下工具会返回可选截图：`create_object`、`edit_object`、`delete_object`、`execute_code`、
`insert_part_from_library`、`get_objects`、`get_object`、`run_fem_analysis`。

| 参数 | 默认值 | 用途 |
| --- | --- | --- |
| `include_screenshot` | `true` | 设为 `false` 只返回文本，适合纯分析脚本或中间步骤。 |
| `view_name` | `"Isometric"` | 截图视角，例如 `"Front"`、`"Top"`、`"Right"`。 |

[`--only-text-feedback`](#纯文本反馈与截图) 会压制这些可选截图，且不受 `include_screenshot` 影响。

需要显式截图时使用 `get_view`，即使开启 `--only-text-feedback` 也可用；它接受 `view_name`
以及可选的 `width`、`height`、`focus_object`。支持的视角为 `Isometric`、`Front`、`Top`、`Right`、
`Back`、`Left`、`Bottom`、`Dimetric`、`Trimetric`。

## FEM 分析

`run_fem_analysis` 对已有的 `Fem::FemAnalysis` 容器运行 CalculiX 求解器。若分析中没有求解器，
会自动创建 `SolverCcxTools`，并返回最大 von Mises 应力、最大位移、节点数与求解器工作目录。
`timeout` 默认 600 秒。长时间分析请让客户端放宽
[队列与执行超时预算](#gui-调度超时)。

端到端示例（几何、材料、网格、约束以及与解析解对比）见上游仓库的
[`examples/cantilever_fem.py`](https://github.com/neka-nat/freecad-mcp/blob/main/examples/cantilever_fem.py)。

## 代码执行

### 三种执行模式

| 工具 | 适用场景 |
| --- | --- |
| `execute_code` | 常规 FreeCAD 自动化，脚本在 GUI 线程执行。 |
| `execute_code_async` | 独立几何上的长时间后台计算；读写文档与视图交给 GUI 线程的 `commit()`。 |
| `execute_code_headless` | 重量级 OCCT 运算放在独立进程中，把原生崩溃与 GUI 隔离。 |

### 共享脚本命名空间

`execute_code` 与 `execute_code_async` 共享一个持久的脚本命名空间，并提供 `FreeCAD`/`App` 与
`FreeCADGui`/`Gui` 别名。变量在多次调用之间保留，且不会覆盖 RPC server 自身的函数。
并发脚本共享同一份实时变量，需要自行协调对同一数据的写入。headless 脚本在全新进程中运行，
不共享该命名空间。

### 后台任务

`execute_code_async` 返回 `job_id`。`get_async_status(job_id)` 报告任务是 `running`、`done`
还是 `failed`，失败时附带异常与堆栈。

所有运行中的任务以及最近 20 个已完成任务会保留在内存中直到 FreeCAD 退出。
`get_async_status()` 列出这段历史，`get_rpc_status` 列出仍在运行的任务 ID。
任务状态不等待 GUI 清理；脚本执行成功也不代表几何体有效。

需要更新后的 addon 才能使用任务状态。若 addon 较旧，请继续轮询文档中的状态对象并查看
FreeCAD 的 Report View。

#### 文档与视图访问

异步代码必须把文档与视图操作留在 GUI 线程：在 worker 中构建独立的 OCCT 形状，
再用 `commit(fn, timeout=120)` 把结果写回并在 GUI 线程重算文档。该辅助函数返回 `fn` 的返回值，
失败时抛出 `RuntimeError`。`commit()` 会保存在共享命名空间中，后续异步调用可复用；
在 `execute_code` 中或 GUI 回调内调用它会立即报错。

### Headless 执行

`execute_code_headless` 把脚本写入文件后以 `freecadcmd -c` 在独立进程中运行。适合可能
segfault 或长时间阻塞 GUI 的 OpenCascade 工作：`makeHelix` + `makePipeShell` 螺纹、loft 与 sweep、
以及带大量 B 样条工具的布尔运算。原生崩溃只会结束辅助进程，工具会报告信号（如 `SIGSEGV`）
以及脚本打印的全部内容，GUI 中的文档不受影响。

脚本需要自行 import 所需模块，并自行打开与保存文档（`FreeCAD.openDocument`、`doc.save()`、
`doc.saveAs()`，或 `Shape.exportBrep`）。保存了 GUI 中已打开的 `.FCStd` 后，用
`reload_document(doc_name)` 刷新 GUI 中的副本；文档名可通过 `list_documents` 获取。

可执行文件在 **MCP server 所在机器**上运行，`--host` 只选择 GUI RPC 主机，因此请使用该机器上可访问的路径。
`timeout` 必须是正有限数（默认 600 秒）：超时会返回部分 stdout/stderr；临时脚本在成功、失败、
超时三种情况下都会被删除（临时脚本目录为 `~/.cache/freecad-mcp/headless`，选择家目录是因为
Flatpak 沙箱通常看不到 `/tmp`）。

可执行文件按顺序自动探测：`freecadcmd`、`FreeCADCmd`、`freecadcmd.exe`、`freecad.cmd`（PATH），
然后是 `org.freecad.FreeCAD` Flatpak。可用 `--freecadcmd` 覆盖：

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

### GUI 调度超时

GUI 线程操作按 FIFO 串行执行，调用有独立的队列预算与执行预算：执行时间从调用真正在
GUI 线程上开始的那一刻计时，等待更早的操作不消耗该预算。队列预算默认等于执行预算；
若在调用开始前耗尽，该调用会被取消且不会稍后补跑，这不会把调度标记为卡住。

| 操作 | 队列预算 | 执行预算 | Go 客户端 socket 超时 |
| --- | --- | --- | --- |
| `execute_code` | 90 秒 | 90 秒 | 至少 210 秒 |
| `run_fem_analysis` | 请求的 `timeout` | 请求的 `timeout` | 至少 `2 * timeout + 30` 秒 |

客户端超时覆盖队列与执行两个预算再加 30 秒余量（基础超时 150 秒，取两者中的较大值）。
其他 MCP 客户端／宿主也需要允许这样的响应时间。并发的 `execute_code` 会排队，但仍串行执行，
因此总耗时是各次运行之和。

### 恢复卡住的 GUI 操作

如果某个 GUI 线程操作在开始后超出执行预算，桥接层返回 `GUI_DISPATCH_STUCK` 并立即拒绝后续 GUI 操作；
已经排队的调用会继续等待（最多到各自的队列超时），并在卡住的操作返回后执行。

用 `get_rpc_status` 可以在不排队的情况下定位仍在运行的操作 —— RPC server 并发处理连接，
本实现中每个 HTTP 请求独立占用连接，因此 `execute_code` 阻塞时 `get_rpc_status`
与 `get_async_status` 依然能立即返回。文档查询（`get_object`、`get_objects`、`list_documents`）
与建模操作一样走 GUI 线程，调度超时或卡住时会返回 RPC fault。

FreeCAD 的 GUI 工作无法安全地强制取消。若操作结束后状态仍未回到 `healthy`，请重启 FreeCAD。

在 FreeCAD 开发版上 `execute_code` 抛异常后，先检查新出现的 `FeaturePython` 对象再修改或删除它；
尤其是 `Proxy` 未成功安装的对象，一旦触碰可能拖死 GUI 线程。

## 项目结构

```
freecad-go-mcp/
├── addon/FreeCADMCP/           # FreeCAD 插件（取自上游，原样分发，不参与 Go 构建）
├── assets/                     # 文档截图与演示 GIF
├── docs/                       # 安装、配置、工具、代码执行、示例文档（英文 + .zh 中文）
├── .github/workflows/test.yml  # CI：gofmt / go vet / go build / go test
├── main.go                     # CLI 参数校验、状态初始化、MCP stdio 启动
├── main_test.go                # --host 校验用例
└── internal/
    ├── xmlrpc/                 # XML-RPC 编解码（<nil/>、struct、array、int/double、fault、base64）
    ├── freecad/
    │   ├── client.go           # RPC 客户端：端口 9875 /RPC2、超时预算、结果类型转换
    │   └── headless.go         # freecadcmd 探测、shell 词法解析、崩溃/超时判定与输出清洗
    └── tools/
        ├── tools.go            # 工具与 prompt 注册、参数读取（缺失与空值区分）
        ├── operations.go       # 17 个工具的具体行为与文案
        ├── descriptions.go     # 工具描述
        ├── prompt.go           # asset_creation_strategy 提示词
        ├── responses.go        # 文本 / JSON / 截图结果封装
        └── state.go            # 配置与持久连接的惰性初始化
```

每个包内都有对应的 `*_test.go`，与实现放在一起。

## 与 Python 实现的一致性

重写时对齐了以下可观察行为：

- **工具集合与文案**：17 个工具 + 1 个 prompt，成功/失败文案与 Python 的
  `operations/core.py` 逐字一致；JSON 输出使用 `indent=2`、不转义 HTML、保留 UTF-8
  （Go 的 map 键按字典序输出，Python 按插入序，这是唯一有意的差异）。
- **超时预算**：见 [GUI 调度超时](#gui-调度超时)，与 `tests/test_client_timeouts.py` 的断言一致。
- **连接与并发**：连接在首次使用时惰性创建并 ping，失败返回
  `Failed to connect to FreeCAD. Make sure the FreeCAD addon is running.`，
  并以 `isError=true` 的工具结果返回（而不是 MCP 协议级错误），与 Python 的 FastMCP 行为一致。
- **截图失败不致命**：截图失败只记录日志并按“无截图”处理，不影响工具主结果。
- **无 Python 运行时依赖**：MCP server 侧不需要 Python；只有 `execute_code_headless`
  会去调用 `freecadcmd`（或你通过 `--freecadcmd` 指定的命令）。

## 交叉编译与发布

`make dist` 会把静态二进制交叉编译到 `dist/` 目录。构建统一使用 `CGO_ENABLED=0`，
因此不依赖任何外部库：

| 系统 | 架构 |
| --- | --- |
| Linux | amd64（x86_64）、arm64 |
| macOS | amd64（Intel）、arm64（Apple 芯片） |
| Windows | amd64（x86_64）、arm64 |

```bash
make dist       # dist/freecad-go-mcp_<版本>_<系统>_<架构>[.exe]，共 6 个二进制
make checksums  # 额外生成 dist/SHA256SUMS
make check      # gofmt + go vet + go test
make clean      # 删除 dist/ 与本机二进制
```

`dist/` 已被 git 忽略。版本号来自 `git describe --tags`，并通过 `-ldflags` 写入二进制，
因此发布前请先给提交打 tag。

推送 `v*` 标签会触发 [.github/workflows/release.yml](.github/workflows/release.yml)：
先执行 `make check`，再交叉编译 6 个二进制，校验 Linux 产物为静态链接，最后连同
`SHA256SUMS` 一起发布为 GitHub Release。在 Actions 页面手动触发同一工作流时，产物会
作为 build artifact 上传，而不会创建 Release。

## 测试

```bash
git clone https://github.com/cnPauLi/freecad-go-mcp.git
cd freecad-go-mcp
go test ./...
```

测试使用 `httptest` 搭建假 XML-RPC 服务端，覆盖：XML-RPC 编解码（含 fault 与指针/命名类型）、
超时预算、阻塞调用下的并发可用性、headless 的成功/失败/崩溃/超时路径（用 `python3` 代替
`freecadcmd` 真实运行脚本）、各工具的文案与截图策略、工具注册表以及 CLI 参数校验。
需要联网拉依赖时同样建议先导出 `GOPROXY`。

## 相关文档

| 文档 | 内容 |
| --- | --- |
| [安装指南](docs/installation.zh.md) | 插件目录、安装截图、构建与客户端配置 |
| [配置](docs/configuration.zh.md) | 自动启动、文本反馈、远程连接 |
| [工具](docs/tools.zh.md) | 工具清单、截图、FEM 分析 |
| [代码执行](docs/execution.zh.md) | GUI 执行、后台任务、headless 脚本、超时排查 |
| [示例](docs/examples.zh.md) | 设计演示、FEM 示例、ADK 与 LangChain 集成 |
| [上游项目](https://github.com/neka-nat/freecad-mcp) | 原始 Python 实现与 addon 来源 |