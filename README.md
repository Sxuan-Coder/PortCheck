# PortCheck

PortCheck 是一个用 Wails v3 写的 Windows 本地端口查看器。

它的目标很简单：当本地服务启动失败、提示端口被占用时，不用再来回敲 `netstat`、`findstr`、`tasklist`，直接打开一个小窗口看清楚端口被哪个进程占了。

这个项目也是公众号文章的配套源码：

> 本地服务端口被谁占了？我用 Wails 做了个 Windows 端口查看器

## 功能

- 查看本机 TCP / UDP 端口。
- 显示本地地址、远程地址、TCP 状态、PID、进程名和进程路径。
- 支持按端口、进程名、PID、地址、路径搜索。
- 支持按协议和 TCP 状态筛选。
- 支持手动刷新端口列表。
- 支持确认后结束占用端口的进程。

## 截图

后续文章发布时会补充运行截图。当前代码可以直接本地运行。

## 安全说明

PortCheck 里的“结束进程”不是关闭某一个端口，而是结束占用这个端口的进程。

项目里做了几个基础保护：

- 不允许结束 PID `0`。
- 不允许结束 PID `4`。
- 不允许结束 PortCheck 自己的进程。
- 每次结束进程前都会弹出确认框。

有些系统进程或管理员权限进程即使点了结束，也可能因为权限不足而失败，这是正常情况。

## 环境要求

当前项目主要在 Windows 上验证。

- Windows 10 / Windows 11
- Go 1.25 或更新版本
- Node.js 24 或更新版本
- npm 11 或更新版本
- Wails v3 CLI

安装 Wails v3 CLI 后，可以先检查版本：

```powershell
wails3 version
```

## 本地运行

```powershell
git clone https://github.com/Sxuan-Coder/PortCheck.git
cd PortCheck

cd frontend
npm install
cd ..

wails3 dev
```

## 打包

```powershell
wails3 task build
```

Windows 下默认产物在：

```text
bin/PortCheck.exe
```

## 测试

```powershell
cd frontend
npm install
npm run build
cd ..

go test ./...
```

这里先构建前端，是因为 Go 入口里会通过 `embed` 打包 `frontend/dist`。

## 当前实测环境

以下结果来自我的 Windows 环境，只代表当前 Demo 和当前打包配置。

| 项目 | 版本 / 数据 |
|---|---|
| 操作系统 | Windows 11 Home Chinese, Build 26200 |
| CPU | AMD Ryzen 7 8745H |
| Go | go1.25.0 windows/amd64 |
| Node.js | v24.14.1 |
| npm | 11.11.0 |
| Wails | v3.0.0-alpha.78 |
| 打包产物 | `PortCheck.exe` |
| 产物体积 | 8.75 MB |

## 项目结构

```text
.
├── main.go                    # Wails 应用入口
├── portservice_windows.go      # Windows 端口和进程查询逻辑
├── portservice_other.go        # 非 Windows 平台占位实现
├── portservice_windows_test.go # Windows 端口解析相关测试
├── frontend                    # Vue 前端
├── build                       # Wails 构建配置
└── Taskfile.yml                # Wails 任务入口
```

## 说明

这个项目是一个最小可用版本，重点是把“端口被谁占了”这件事查清楚。

目前没有做后台常驻、端口变化监听、批量结束进程、导出报告这些功能。后续如果继续写文章，我会优先按真实使用场景往下加，而不是把功能一次堆满。

## 致谢

本项目基于 [fengfengzhidao/port_lite](https://github.com/fengfengzhidao/port_lite) 改造而来，感谢原作者 **枫枫** 的开源贡献。
