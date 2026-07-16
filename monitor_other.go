//go:build !windows

package main

import "time"

// readCPUInfo 非 Windows 占位：返回空值。
func readCPUInfo() (int, string) {
	return 0, ""
}

// sampleProcessesAndPerf 非 Windows 占位：不采集任何数据。
func sampleProcessesAndPerf(_ *MonitorService, _ float64) ([]ProcessInfo, PerfSnapshot) {
	return []ProcessInfo{}, PerfSnapshot{}
}

// portCountsLight 非 Windows 占位：返回空统计。
func portCountsLight() PortListResult {
	return PortListResult{}
}

// 引用 time 包以避免在无采样调用时被标记为未使用（保留接口语义）。
var _ = time.Second
