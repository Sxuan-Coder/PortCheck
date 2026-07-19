//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveStartupExePathQuoted(t *testing.T) {
	got := resolveStartupExePath(`"C:\Program Files\App\app.exe" --flag`)
	want := `C:\Program Files\App\app.exe`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveStartupExePathUnquotedExisting(t *testing.T) {
	dir := t.TempDir()
	// 含空格的路径
	sub := filepath.Join(dir, "Program Files", "App")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(sub, "tool.exe")
	if err := os.WriteFile(exe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exe + " /background"
	got := resolveStartupExePath(cmd)
	if got != exe {
		t.Fatalf("got %q want %q", got, exe)
	}
}

func TestIconDataURLForPathSyncMissing(t *testing.T) {
	url := iconDataURLForPathSync(`C:\this\path\does\not\exist-portcheck.exe`)
	if url == "" {
		t.Fatal("expected fallback app icon data URL, got empty")
	}
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("unexpected fallback: %q", url[:min(40, len(url))])
	}
}

func TestFallbackAppIconDataURL(t *testing.T) {
	if len(appIcon) == 0 {
		t.Skip("appIcon embed empty in this package build context")
	}
	url := fallbackAppIconDataURL()
	if url == "" || !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("bad fallback: %q", url)
	}
}

func TestIconDataURLForPathSyncSelf(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// go test 的可执行文件通常有图标资源；即使为空也不应 panic。
	_ = iconDataURLForPathSync(exe)
}
