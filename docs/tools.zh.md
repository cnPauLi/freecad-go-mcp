# 工具

[English](tools.md) · [中文](tools.zh.md) · [返回 README](../README.zh.md) · [配置](configuration.zh.md) · [代码执行](execution.zh.md)

## 可用工具

| 工具 | 用途 |
| --- | --- |
| `create_document` | 创建一个新的 FreeCAD 文档。 |
| `list_documents` | 列出已打开的文档。 |
| `reload_document` | 关闭并重新打开已保存的文档，以获取外部文件变更，例如无界面脚本产生的结果。 |
| `create_object` | 在文档中创建一个对象。 |
| `edit_object` | 编辑对象的属性。 |
| `delete_object` | 从文档中删除一个对象。 |
| `get_objects` | 获取文档中的所有对象。 |
| `get_object` | 获取文档中的某一个对象。 |
| `get_view` | 获取当前活动视角的截图。 |
| `execute_code` | 在 FreeCAD 的 GUI 线程上执行 Python 代码。 |
| `execute_code_async` | 启动一个后台计算并返回其任务 ID；使用 `commit()` 访问文档和视图。 |
| `get_async_status` | 在不使用 GUI 线程的情况下获取后台任务状态和失败回溯信息。 |
| `execute_code_headless` | 在独立的 `freecadcmd` 进程中运行脚本，并返回其退出状态和输出。 |
| `get_rpc_status` | 在不使用 GUI 线程的情况下报告 RPC 与 GUI 调度的健康状况。 |
| `insert_part_from_library` | 从 [FreeCAD 零件库](https://github.com/FreeCAD/FreeCAD-library) 中插入一个零件。 |
| `get_parts_list` | 列出 [FreeCAD 零件库](https://github.com/FreeCAD/FreeCAD-library) 中的零件。 |
| `run_fem_analysis` | 对已有分析运行 CalculiX，并返回汇总结果。 |

关于执行模式、共享脚本状态、后台任务跟踪和超时处理，请参阅[代码执行](execution.zh.md)。

## 截图选项

以下工具会返回可选截图：`create_object`、`edit_object`、
`delete_object`、`execute_code`、`insert_part_from_library`、`get_objects`、
`get_object` 和 `run_fem_analysis`。

| 参数 | 默认值 | 用途 |
| --- | --- | --- |
| `include_screenshot` | `true` | 设为 `false` 可仅返回文本反馈，例如纯分析脚本或中间步骤。 |
| `view_name` | `"Isometric"` | 调整返回截图的视角，例如 `"Front"`、`"Top"` 或 `"Right"`。 |

无论 `include_screenshot` 如何设置，[`--only-text-feedback` 标志](configuration.zh.md#文本反馈与截图)
都会抑制这些可选截图。

使用 `get_view` 可以显式请求截图；即使在
`--only-text-feedback` 下它依然可用。它接受 `view_name` 以及可选的 `width`、`height` 和
`focus_object` 参数。支持的视角有 `Isometric`、`Front`、`Top`、
`Right`、`Back`、`Left`、`Bottom`、`Dimetric` 和 `Trimetric`。

## FEM 分析

`run_fem_analysis` 对已有的 `Fem::FemAnalysis`
容器运行 CalculiX 求解器。如果该分析没有求解器，它会自动创建一个 `SolverCcxTools`，并返回
最大 von Mises 应力、最大/最小位移、节点数以及求解器的工作
目录。`timeout` 的默认值为 600 秒。

关于端到端示例（包括几何、材料、网格、约束以及解析对比），请参阅上游仓库中的
[`examples/cantilever_fem.py`](https://github.com/neka-nat/freecad-mcp/blob/main/examples/cantilever_fem.py)。
对于耗时较长的分析，请配置客户端以允许
[队列与执行超时预算](execution.zh.md#gui-调度超时)。