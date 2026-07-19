# AGENTS.md

这个仓库是 PortCheck，一个用 Wails v3 + Vue 写的 Windows 本地任务管理与端口查看工具（FastTaskManager 形态）。

后续在这个仓库里改代码时，优先遵守下面这些约定。

## 项目定位

- 仍聚焦开发者本地排查：端口占用、进程 / 性能 / 服务 / 启动项一览。
- 端口是核心完整能力；进程/性能为完整能力；**服务 v2 支持停止/启动**，**启动项 v2 支持删除/禁用/启用**（危险操作需二次确认；权限不足时按需 UAC 提权）。
- 新功能最好服务开发者的真实场景，比如哪个端口被哪个进程占了、nodejs/Java/go 进程没关闭一直后台运行。

## 后端文件地图（Go，Windows / 非 Windows 拆分）

- `portservice_windows.go` / `portservice_other.go`：端口查询与结束进程（`ListPorts` / `KillProcess`）。
- `processinfo_windows.go`：进程名/路径查询助手（`queryProcessInfo`），端口采样与进程采样共用。
- `monitor.go` + `monitor_windows.go` + `monitor_other.go`：`MonitorService` 单 ticker（1s）批量推送 `monitor:tick` 事件，内含进程 CPU/内存、整机 CPU/内存、端口统计；CPU% 基于两次采样差值。
- `services_windows.go` / `services_startup_other.go`：`ServicesService` 枚举 + 停止/启动（关键服务保护名单 + 先试后提权）。
- `startup_windows.go` / `services_startup_other.go`：`StartupService` 枚举（含 StartupApproved 禁用态）+ 删除/禁用/启用。
- `elevate_windows.go` / `elevate_other.go`：按需 UAC 提权（`ShellExecuteEx runas` + 结果文件协议）与 `--elevated` CLI 分发。
- `icon_windows.go` / `icon_other.go`：从 exe/lnk 提取应用图标（SHGetFileInfo → PNG base64）；启动项同步填充，进程 tick 异步缓存后按路径只推送一次。
- `models.go`：跨平台共享结构体（`ProcessInfo` / `PerfSnapshot` / `ServiceEntry` / `StartupEntry` / `ServiceOpResult` / `StartupOpResult` / `MonitorTick` 等）。
- `main.go`：提权子进程早期分支 + 无边框窗口 + 系统托盘 + 服务注册。

## 前端文件地图（Vue3 + 纯 CSS）

- 高性能约定：`monitor:tick` 全生命周期只订阅一次（`composables/useMonitor.ts`）；大数组用 `markRaw` / `shallowReactive` 关闭深响应；进程表用虚拟滚动（`useVirtualList`）；Canvas 图表 RAF 节流（`components/MiniChart.vue`）；滚动区与表格行**不加 `backdrop-filter`**。
- `tabs/`：Processes（虚拟列表+结束进程二次确认）、Performance、Ports（端口完整能力）、Services（停止/启动二次确认）、Startup（删除/禁用/启用二次确认）。
- 样式为纯 CSS + 设计 token（`src/assets/theme.css`），**不引 Tailwind / UI 组件库**；图标为内联 SVG（`components/AppIcon.vue`）。
- 标题栏拖拽靠 CSS 变量 `--wails-draggable: drag`（runtime 自动处理）。

## 开发约定

- 非 Windows 平台只保留占位实现，不要假装已经完整支持。
- 前端尽量保持轻量，不引入大型 UI 组件库（当前为纯 CSS）。
- 结束进程属于危险操作，必须保留明确确认（`Dialogs.Question`），不要做一键批量结束。
- PID `0`、PID `4`、当前应用自身进程必须继续保护（`validateKillPID`）。
- 涉及进程路径、用户名、公司目录的截图或文章素材要先脱敏。
- 修改 Go 服务方法后，`wails3 task build` 会自动 `wails3 generate bindings -ts` 重新生成 `frontend/bindings`（`.ts`）；勿手改生成产物。

## 经验教训

### 版本号批量替换时勿伤 lockfile 依赖版本

- **触发信号：** 用 `sed` 全局替换版本号（如 `2.2.0 → 2.2.1`）后，`npm install` 因 npmmirror 404 失败。
- **根因：** `package-lock.json` 嵌套了大量依赖自身的 `version` 字段（如 `brace-expansion` 的版本 `2.2.0`），全局替换误改了它们。
- **正确做法：** `sed` 替换时先用 `git grep "2\.2\.0"` 列出所有命中位置，排除 lockfile 中的依赖包版本；或只替换 `update.go`、`package.json`、`build/config.yml` 等明确的应用版本号文件，lockfile 用 `npm install` 自动更新。
- **验证方式：** 替换后 `npm install` 应无报错，`git diff package-lock.json` 检查只有顶层 `version` 被改。

### Git 提交信息必须使用中文

- **触发信号：** 子代理自动提交时使用了英文 commit message（如 `feat: add Settings struct`），不符合仓库惯例。
- **根因：** 子代理默认使用英文，未遵循本仓库 `commit 规范` 中 `summary 使用中文` 的要求。
- **正确做法：** `summary` 使用中文、动词开头、长度 ≤ 50 字、不加句号。格式 `<type>(scope): 中文描述`。
- **验证方式：** 推送前 `git log --oneline` 检查每条 message 是否为中文。

## 验证命令

修改后优先跑：

```powershell
cd frontend
npm install
npm run build
cd ..

go test ./...
wails3 task build
```

如果只是改文档，可以不跑完整构建，但要说明没有运行。

