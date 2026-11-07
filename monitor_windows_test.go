//go:build windows

package main

import "testing"

func TestIoRate(t *testing.T) {
	// 正常差值：200 字节增量 / 2 秒 = 100 B/s。
	if got := ioRate(300, 100, 2); got != 100 {
		t.Fatalf("ioRate(300,100,2) = %v, want 100", got)
	}
	// 首次见到（prev=0）返回 0，避免把累计 I/O 当单 tick 增量。
	if got := ioRate(1<<30, 0, 1); got != 0 {
		t.Fatalf("ioRate first-seen = %v, want 0", got)
	}
	// 计数器回退（PID 复用）返回 0。
	if got := ioRate(50, 100, 1); got != 0 {
		t.Fatalf("ioRate rollback = %v, want 0", got)
	}
	// 非法 elapsed 返回 0。
	if got := ioRate(300, 100, 0); got != 0 {
		t.Fatalf("ioRate elapsed=0 = %v, want 0", got)
	}
}
