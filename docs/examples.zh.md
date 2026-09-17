# 演示与示例

[English](examples.md) · [中文](examples.zh.md) · [返回 README](../README.zh.md) · [安装](installation.zh.md) · [工具](tools.zh.md)

## 设计演示

### 设计法兰

![在 FreeCAD 中设计法兰](../assets/freecad_mcp4.gif)

### 设计玩具车

![在 FreeCAD 中设计玩具车](../assets/make_toycar4.gif)

### 从 2D 图纸建模

输入图纸：

![输入的 2D 图纸](../assets/b9-1.png)

演示：

![根据图纸对零件建模](../assets/from_2ddrawing.gif)

[对话记录](https://claude.ai/share/7b48fd60-68ba-46fb-bb21-2fbb17399b48)

## 示例脚本

这些脚本位于上游仓库中，运行在 FreeCAD 插件之上；
它们与 MCP server 使用何种语言编写无关。

| 示例 | 说明 |
| --- | --- |
| [悬臂梁有限元分析](https://github.com/neka-nat/freecad-mcp/blob/main/examples/cantilever_fem.py) | 构建悬臂梁，运行 CalculiX，并将结果与解析解进行比较。 |
| [Google ADK agent](https://github.com/neka-nat/freecad-mcp/blob/main/examples/adk/agent.py) | 使用本地代码副本将 ADK agent 连接到 MCP server。 |
| [LangChain / LangGraph agent](https://github.com/neka-nat/freecad-mcp/blob/main/examples/langchain/react.py) | 使用 MCP 工具和 Groq 模型运行交互式 CAD agent。 |

agent 示例使用了可选的第三方依赖和服务商配置。
运行每个示例之前，请调整其中的仓库路径和模型设置；LangChain 示例还要求环境变量中存在 `GROQ_API_KEY`。
两个 agent 示例都会自行启动 MCP server 进程，因此请让它们指向由本仓库构建的 Go 二进制文件
（参见[安装](installation.zh.md)），而不是 Python 入口点。