//go:build !windows

package main

import "errors"

// isElevatedHelper 非 Windows 始终返回 false。
func isElevatedHelper(args []string) bool {
	return false
}

// runElevatedHelper 非 Windows 占位。
func runElevatedHelper(args []string) {
	// no-op
}

// tryDirectOrElevate 非 Windows 直接返回不支持。
func tryDirectOrElevate(directFn func() error, elevateOp, name, location string) error {
	return errors.New("提权写操作目前只支持 Windows")
}

// isElevated 非 Windows 占位：始终返回 false。
func isElevated() bool {
	return false
}

// relaunchElevated 非 Windows 占位：不支持。
func relaunchElevated() error {
	return errors.New("提权重启目前只支持 Windows")
}

// ensureProcessScopeElevation 非 Windows 占位：不处理。
func ensureProcessScopeElevation() {}
