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
