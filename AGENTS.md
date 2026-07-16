# AGENTS.md

这个仓库是 PortCheck，一个用 Wails v3 + Vue 写的 Windows 本地任务管理与端口查看工具（FastTaskManager 形态）。

后续在这个仓库里改代码时，优先遵守下面这些约定。

## 项目定位

- 仍聚焦开发者本地排查：端口占用、进程 / 性能 / 服务 / 启动项一览。
- 端口是核心完整能力；进程/性能/服务/启动项中，**服务与启动项 v1 为只读**（仅列表，不做启停/启用禁用写操作）。
- 新功能最好服务开发者的真实场景，比如哪个端口被哪个进程占了、nodejs/Java/go 进程没关闭一直后台运行。

## 后端文件地图（Go，Windows / 非 Windows 拆分）

- `portservice_windows.go` / `portservice_other.go`：端口查询与结束进程（`ListPorts` / `KillProcess`）。
- `processinfo_windows.go`：进程名/路径查询助手（`queryProcessInfo`），端口采样与进程采样共用。
- `monitor.go` + `monitor_windows.go` + `monitor_other.go`：`MonitorService` 单 ticker（1s）批量推送 `monitor:tick` 事件，内含进程 CPU/内存、整机 CPU/内存、端口统计；CPU% 基于两次采样差值。
- `services_windows.go` / `services_startup_other.go`：`ServicesService.ListServices` 只读枚举（单次 `EnumServicesStatusEx`）。
- `startup_windows.go` / `services_startup_other.go`：`StartupService.ListStartup` 只读枚举（注册表 Run 键 + Startup 文件夹）。
- `models.go`：跨平台共享结构体（`ProcessInfo` / `PerfSnapshot` / `ServiceEntry` / `StartupEntry` / `MonitorTick` 等）。
- `main.go`：无边框窗口 + 系统托盘（关窗到托盘）+ 四个服务注册。

## 前端文件地图（Vue3 + 纯 CSS）

- 高性能约定：`monitor:tick` 全生命周期只订阅一次（`composables/useMonitor.ts`）；大数组用 `markRaw` / `shallowReactive` 关闭深响应；进程表用虚拟滚动（`useVirtualList`）；Canvas 图表 RAF 节流（`components/MiniChart.vue`）；滚动区与表格行**不加 `backdrop-filter`**。
- `tabs/`：Processes（虚拟列表+结束进程二次确认）、Performance、Ports（端口完整能力）、Services（只读）、Startup（只读）。
- 样式为纯 CSS + 设计 token（`src/assets/theme.css`），**不引 Tailwind / UI 组件库**；图标为内联 SVG（`components/AppIcon.vue`）。
- 标题栏拖拽靠 CSS 变量 `--wails-draggable: drag`（runtime 自动处理）。

## 开发约定

- 非 Windows 平台只保留占位实现，不要假装已经完整支持。
- 前端尽量保持轻量，不引入大型 UI 组件库（当前为纯 CSS）。
- 结束进程属于危险操作，必须保留明确确认（`Dialogs.Question`），不要做一键批量结束。
- PID `0`、PID `4`、当前应用自身进程必须继续保护（`validateKillPID`）。
- 涉及进程路径、用户名、公司目录的截图或文章素材要先脱敏。
- 修改 Go 服务方法后，`wails3 task build` 会自动 `wails3 generate bindings -ts` 重新生成 `frontend/bindings`（`.ts`）；勿手改生成产物。

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

