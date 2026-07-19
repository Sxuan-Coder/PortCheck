//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureServiceStopAllowed(t *testing.T) {
	if err := ensureServiceStopAllowed("RpcSs"); err == nil {
		t.Fatal("RpcSs should be protected")
	}
	if err := ensureServiceStopAllowed("Spooler"); err != nil {
		t.Fatalf("Spooler should be allowed: %v", err)
	}
}

func TestParseElevatedArgs(t *testing.T) {
	out, op, name, loc, err := parseElevatedArgs([]string{
		"PortCheck.exe", "--elevated", "--out", "C:\\tmp\\r.json",
		"--op", "startup-disable", "--name", "Foo", "--location", "HKCU",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != `C:\tmp\r.json` || op != "startup-disable" || name != "Foo" || loc != "HKCU" {
		t.Fatalf("parsed = %q %q %q %q", out, op, name, loc)
	}

	if _, _, _, _, err := parseElevatedArgs([]string{"x", "--elevated", "--op", "ping"}); err == nil {
		t.Fatal("missing --out should fail")
	}
}

func TestIsElevatedHelper(t *testing.T) {
	if !isElevatedHelper([]string{"app", "--elevated", "--op", "ping"}) {
		t.Fatal("expected elevated")
	}
	if isElevatedHelper([]string{"app"}) {
		t.Fatal("expected not elevated")
	}
}

func TestValidateStartupArgs(t *testing.T) {
	if err := validateStartupArgs("", "HKCU"); err == nil {
		t.Fatal("empty name should fail")
	}
	if err := validateStartupArgs("x", "BAD"); err == nil {
		t.Fatal("bad location should fail")
	}
	if err := validateStartupArgs("x", "HKCU"); err != nil {
		t.Fatal(err)
	}
}

func TestStartupApprovedRoundTripHKCU(t *testing.T) {
	// 仅在可写 HKCU 时执行；写入一个临时测试名，验证后清理。
	name := "PortCheckTestStartup_" + filepath.Base(t.TempDir())
	// 先确保 Run 键有值（禁用依赖值名存在于 StartupApproved，不依赖 Run 值本身）。
	// 这里只测 approved 读写。
	if err := disableStartupDirect(name, "HKCU"); err != nil {
		t.Skipf("cannot write StartupApproved (skip): %v", err)
	}
	t.Cleanup(func() {
		_ = enableStartupDirect(name, "HKCU")
		// 清理 approved 值。
		_ = deleteApprovedValue(0x80000001 /* CURRENT_USER */, name, "Run")
	})

	if !isStartupDisabled(name, "HKCU") {
		t.Fatal("expected disabled after disableStartupDirect")
	}
	if err := enableStartupDirect(name, "HKCU"); err != nil {
		t.Fatal(err)
	}
	if isStartupDisabled(name, "HKCU") {
		t.Fatal("expected enabled after enableStartupDirect")
	}
}

func TestWriteElevatedResultRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.json")
	want := elevatedResult{OK: true, Action: "ping", Target: "x", Message: "pong"}
	if err := writeElevatedResult(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := readElevatedResult(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.Message != "pong" || got.Action != "ping" {
		t.Fatalf("got %+v", got)
	}
	// 确保临时文件可被读。
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
