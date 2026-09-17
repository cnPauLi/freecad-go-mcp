# 代码执行

[English](execution.md) · [中文](execution.zh.md) · [返回 README](../README.zh.md) · [工具](tools.zh.md) · [配置](configuration.zh.md)

## 选择执行模式

| 工具 | 适用场景 |
| --- | --- |
| `execute_code` | 常规 FreeCAD 自动化；脚本在 GUI 线程上运行。 |
| `execute_code_async` | 在独立几何体上进行长时间后台计算；通过 `commit()` 将文档与视图访问交还给 GUI 线程。 |
| `execute_code_headless` | 在独立进程中执行繁重的 OCCT 操作，将原生崩溃与 GUI 隔离。 |

## 共享脚本命名空间

`execute_code` 和 `execute_code_async` 共享一个持久化的脚本命名空间，其中带有
`FreeCAD`/`App` 和 `FreeCADGui`/`Gui` 别名。脚本变量在多次调用之间保留，且不会覆盖
RPC 服务器自身的函数。这可以避免意外的名称冲突；代码执行仍然拥有 FreeCAD 的全部权限。

并发脚本共享实时变量，对同一数据的写入必须有意识地加以协调。Headless 脚本在全新的
进程中运行，不共享该命名空间。

## 后台任务

`execute_code_async` 会返回一个 `job_id`。`get_async_status(job_id)` 会报告任务处于
`running`、`done` 还是 `failed` 状态，并对失败的任务给出异常和堆栈。

所有正在运行的任务以及最近完成的 20 个任务都会保留在内存中，直到 FreeCAD 退出。
`get_async_status()` 会列出这一历史记录；`get_rpc_status` 会列出仍在运行的任务 ID。
任务状态不会等待 GUI 清理完成。脚本执行成功并不能保证几何体有效。

请安装更新后的 addon 以使用任务状态。如果 addon 较旧，请继续轮询文档状态对象并查看
FreeCAD 的 Report View。

### 文档与视图访问

异步代码必须将文档与视图访问保持在 GUI 线程上。在工作线程中构建独立的 OCCT 形状，
然后使用 `commit(fn, timeout=120)` 在 GUI 线程上应用结果并重算文档。该辅助进程返回
`fn` 的返回值，或在失败时抛出 `RuntimeError`。

`commit()` 会持久化在共享命名空间中，因此保存下来的函数可以在后续异步调用中复用它。
从 `execute_code` 中或 GUI 回调内部调用它会立即抛出异常。

## Headless 执行

`execute_code_headless` 会将脚本写入文件，并在独立进程中用
`freecadcmd -c` 运行它。它适用于可能触发段错误或让 GUI 阻塞数分钟的 OpenCascade
工作：`makeHelix` + `makePipeShell` 线程、放样和扫掠，或包含大量 B 样条工具的布尔运算。
原生崩溃只会终止辅助进程；该工具会报告信号（例如 `SIGSEGV`）以及脚本打印的全部内容，
而 GUI 会保留其文档。

脚本必须自行导入所需的模块，并自行打开和保存文档
（`FreeCAD.openDocument`、`doc.save()`、`doc.saveAs()`，或用于导出形状的
`Shape.exportBrep`）。在保存 GUI 中已打开的 `.FCStd` 文件后，请使用
`reload_document(doc_name)` 刷新 GUI 中的副本。可通过 `list_documents` 获取文档名称。

可执行文件运行在承载 MCP 服务器的机器上；`--host` 只用于选择 GUI RPC 主机。请使用
在 MCP 服务器机器上可访问的文件路径。超时必须为正数且有限（默认值：600 秒）。超时会
返回部分 stdout/stderr，临时脚本会在成功、失败和超时三种情况下被删除。临时脚本位于
`~/.cache/freecad-mcp/headless`（主目录，因为 Flatpak 沙箱通常看不到 `/tmp`）。

可执行文件按以下顺序自动检测：`PATH` 上的 `freecadcmd`、`FreeCADCmd`、
`freecadcmd.exe` 和 `freecad.cmd`，然后是 `org.freecad.FreeCAD` Flatpak。可通过以下
方式覆盖：

```bash
freecad-go-mcp --freecadcmd "flatpak run --command=freecadcmd org.freecad.FreeCAD"
```

`--freecadcmd` 的值按照 shell 分词规则解析，因此带引号的参数和转义均可用；引号未闭合
会导致启动失败。

## GUI 调度超时

GUI 线程操作按 FIFO 顺序（先进先出）逐个运行。调用具有独立的队列预算和执行预算：
执行时间从调用在 GUI 线程上开始的那一刻起计算，等待更早的操作不会消耗该预算。队列预算
默认等于执行预算。如果队列预算在调用开始之前就已耗尽，该调用会被取消，且之后不会再
运行；这不会把调度标记为卡住。

| 操作 | 队列预算 | 执行预算 | Go 客户端 socket 超时 |
| --- | --- | --- | --- |
| `execute_code` | 90 秒 | 90 秒 | 至少 210 秒 |
| `run_fem_analysis` | 请求的 `timeout` | 请求的 `timeout` | 至少 `2 * timeout + 30` 秒 |

客户端超时覆盖这两项预算再加 30 秒余量，并以 150 秒为基础值：它取 150 秒与上表中
各操作预算二者中的较大者。其他客户端和 MCP 宿主必须在其自身的超时设置中允许这些
响应时间。并发的 `execute_code` 调用可以排队，但它们仍会顺序执行，因此总耗时包含
每一次单独运行的时间。

### 恢复卡住的 GUI 操作

如果 GUI 线程操作在开始后超出了其执行预算，桥接层会返回 `GUI_DISPATCH_STUCK`，
并立即拒绝后续的 GUI 操作。已经排队的调用会继续等待，直至其队列超时，并在卡住的
操作返回后运行。

使用 `get_rpc_status` 可以识别仍在运行的操作。Go 客户端在各自的 HTTP 连接上发起每次
调用，而 RPC 服务器并发处理连接，因此在 `execute_code` 阻塞期间诊断调用会立即返回。
文档查询（`get_object`、`get_objects` 和 `list_documents`）与建模操作一样在 GUI 线程上
运行，如果调度超时或卡住，会报告 RPC 故障。

FreeCAD GUI 的工作无法安全地强制取消。如果操作结束后状态仍未恢复到 `healthy`，
请重启 FreeCAD。

在 FreeCAD 开发版上出现 `execute_code` 异常后，在修改或删除任何新的
`FeaturePython` 对象之前请先检查它。特别地，不要继续操作其必需的 `Proxy` 从未安装
过的对象，因为触碰那个损坏的对象可能会卡死 FreeCAD 的 GUI 线程。