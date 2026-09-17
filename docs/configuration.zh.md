# 配置

[English](configuration.md) · [中文](configuration.zh.md) · [返回 README](../README.zh.md) · [安装](installation.zh.md) · [工具](tools.zh.md)

## 自动启动 RPC 服务

默认情况下，每次打开 FreeCAD 时都必须手动启动 RPC 服务。要让它自动启动：

1. 切换到 **MCP Addon** 工作台，打开 **FreeCAD MCP** 菜单。
2. 勾选 **Auto-Start Server**。

该设置会保存到 `freecad_mcp_settings.json` 并持久保存，跨会话生效。
下次启动 FreeCAD 时，应用加载完成后 RPC 服务会自动启动。在同一菜单中取消勾选
**Auto-Start Server** 即可禁用该功能。

## 文本反馈与截图

传入 `--only-text-feedback` 可以从工具反馈中省略可选的截图，并减少 token 消耗。
将 `/path/to/freecad-go-mcp` 替换为你所构建的二进制文件的绝对路径
（参见[安装](installation.zh.md)）：

```json
{
  "mcpServers": {
    "freecad": {
      "command": "/path/to/freecad-go-mcp",
      "args": ["--only-text-feedback"]
    }
  }
}
```

你也可以针对每次调用，通过 `include_screenshot` 和 `view_name` 控制可选截图。
全局标志优先于 `include_screenshot`。
适用的工具以及显式的 `get_view` 工具请参见[截图选项](tools.zh.md#截图选项)。

## 远程连接

默认情况下，RPC 服务监听 `localhost`，不接受远程连接。若要从网络中的另一台机器控制
FreeCAD，需要同时配置插件（addon）和 MCP 客户端。

### 1. 在 FreeCAD 中启用远程连接

在 **FreeCAD MCP** 工具栏中：

1. 勾选 **Remote Connections**。在下次重启服务时，RPC 服务会绑定到
   `0.0.0.0`（所有接口）。它只接受来自 **Allowed IPs** 中配置的 IP 地址
   或 CIDR 子网的连接，该设置默认为 `127.0.0.1`。
2. 点击 **Configure Allowed IPs**，输入以逗号分隔的允许客户端 IP 地址或 CIDR 子网列表，
   例如：

   ```text
   192.168.1.100, 10.0.0.0/24
   ```

   无效条目会被拒绝，并弹出错误对话框。
3. 更改这些设置后，重启 RPC 服务。

### 2. 让 MCP server 连接远程主机

传入 `--host`，其值为运行 FreeCAD 的机器的 IP 地址或主机名：

```json
{
  "mcpServers": {
    "freecad": {
      "command": "/path/to/freecad-go-mcp",
      "args": ["--host", "192.168.1.100"]
    }
  }
}
```

`--host` 的值会在启动时校验，必须是有效的 IPv4/IPv6 地址或主机名。更新 MCP 客户端的
配置后，请重启 MCP 客户端。

`--host` 用于选择 GUI RPC 主机。[Headless 执行](execution.zh.md#headless-执行)
运行在托管 MCP server 的机器上，因此其文件路径必须在该机器上可访问。