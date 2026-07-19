//go:build windows

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	shgfiIcon      = 0x000000100
	shgfiSmallIcon = 0x000000001
	shgfiLargeIcon = 0x000000000
	diNormal       = 0x0003
	biRGB          = 0
	dibRGBColors   = 0
	// iconPixelSize 提取边长；前端缩放到 ~18px，32 在高 DPI 更清晰。
	iconPixelSize = 32
)

type shfileinfo struct {
	hIcon         windows.Handle
	iIcon         int32
	dwAttributes  uint32
	szDisplayName [windows.MAX_PATH]uint16
	szTypeName    [80]uint16
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type bitmapInfo struct {
	bmiHeader bitmapInfoHeader
	bmiColors [1]uint32
}

var (
	modShell32Icon = windows.NewLazySystemDLL("shell32.dll")
	modUser32Icon  = windows.NewLazySystemDLL("user32.dll")
	modGdi32Icon   = windows.NewLazySystemDLL("gdi32.dll")

	procSHGetFileInfoW    = modShell32Icon.NewProc("SHGetFileInfoW")
	procDestroyIcon       = modUser32Icon.NewProc("DestroyIcon")
	procDrawIconEx        = modUser32Icon.NewProc("DrawIconEx")
	procGetDCIcon         = modUser32Icon.NewProc("GetDC")
	procReleaseDCIcon     = modUser32Icon.NewProc("ReleaseDC")
	procCreateCompatibleDC = modGdi32Icon.NewProc("CreateCompatibleDC")
	procDeleteDCIcon      = modGdi32Icon.NewProc("DeleteDC")
	procCreateDIBSection  = modGdi32Icon.NewProc("CreateDIBSection")
	procSelectObjectIcon  = modGdi32Icon.NewProc("SelectObject")
	procDeleteObjectIcon  = modGdi32Icon.NewProc("DeleteObject")
)

// 图标缓存：key 为规范化路径，value 为 data URL（失败缓存为空串）。
var (
	iconCacheMu  sync.Mutex
	iconCache    = map[string]string{}
	iconEmitted  = map[string]bool{} // 进程 tick 每路径只推送一次，前端自行记忆
	iconInflight = map[string]bool{}
)

func normalizeIconPath(path string) string {
	return strings.ToLower(filepath.Clean(path))
}

// iconDataURLForPathSync 同步提取（启动项列表用，一次返回）；失败回退应用 logo。
func iconDataURLForPathSync(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return fallbackAppIconDataURL()
	}
	key := normalizeIconPath(path)
	iconCacheMu.Lock()
	if url, ok := iconCache[key]; ok {
		iconCacheMu.Unlock()
		return url
	}
	iconCacheMu.Unlock()

	url := extractIconDataURL(path)
	iconCacheMu.Lock()
	iconCache[key] = url
	iconCacheMu.Unlock()
	return url
}

// iconDataURLForEmit 进程采样用：缓存命中且未推送过则返回；未缓存则异步提取，本 tick 返回空。
// 提取失败时缓存为应用 logo（非空），前端可直接显示。
func iconDataURLForEmit(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		// 无路径进程（如 System Idle）统一用应用 logo，只推一次。
		key := "__fallback__"
		iconCacheMu.Lock()
		defer iconCacheMu.Unlock()
		if iconEmitted[key] {
			return ""
		}
		url := fallbackAppIconDataURL()
		if url == "" {
			return ""
		}
		iconEmitted[key] = true
		return url
	}
	key := normalizeIconPath(path)

	iconCacheMu.Lock()
	defer iconCacheMu.Unlock()

	url, ok := iconCache[key]
	if !ok {
		if !iconInflight[key] {
			iconInflight[key] = true
			go func(p, k string) {
				u := extractIconDataURL(p)
				iconCacheMu.Lock()
				iconCache[k] = u
				delete(iconInflight, k)
				iconCacheMu.Unlock()
			}(path, key)
		}
		return ""
	}
	if url == "" || iconEmitted[key] {
		return ""
	}
	iconEmitted[key] = true
	return url
}

func extractIconDataURL(path string) string {
	pngBytes, err := extractFileIconPNG(path, iconPixelSize)
	if err != nil || len(pngBytes) == 0 {
		return fallbackAppIconDataURL()
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
}

// fallbackAppIconDataURL 使用嵌入的 PortCheck 应用图标（main.go 的 appIcon）。
func fallbackAppIconDataURL() string {
	if len(appIcon) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(appIcon)
}

// extractFileIconPNG 从文件提取图标并编码为 PNG。
func extractFileIconPNG(path string, size int) ([]byte, error) {
	if size <= 0 {
		size = iconPixelSize
	}
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	var shfi shfileinfo
	flags := uintptr(shgfiIcon | shgfiLargeIcon)
	if size <= 16 {
		flags = uintptr(shgfiIcon | shgfiSmallIcon)
	}
	r, _, callErr := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&shfi)),
		unsafe.Sizeof(shfi),
		flags,
	)
	if r == 0 || shfi.hIcon == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return nil, callErr
		}
		return nil, errors.New("无法提取图标")
	}
	defer procDestroyIcon.Call(uintptr(shfi.hIcon))

	img, err := hiconToRGBA(shfi.hIcon, size)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// hiconToRGBA 将 HICON 绘制到 32bpp DIB 并转为 image.RGBA（BGRA→RGBA）。
func hiconToRGBA(hIcon windows.Handle, size int) (*image.RGBA, error) {
	hdcScreen, _, _ := procGetDCIcon.Call(0)
	if hdcScreen == 0 {
		return nil, errors.New("GetDC 失败")
	}
	defer procReleaseDCIcon.Call(0, hdcScreen)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return nil, errors.New("CreateCompatibleDC 失败")
	}
	defer procDeleteDCIcon.Call(hdcMem)

	var bits unsafe.Pointer
	bi := bitmapInfo{}
	bi.bmiHeader.biSize = uint32(unsafe.Sizeof(bi.bmiHeader))
	bi.bmiHeader.biWidth = int32(size)
	bi.bmiHeader.biHeight = -int32(size) // top-down
	bi.bmiHeader.biPlanes = 1
	bi.bmiHeader.biBitCount = 32
	bi.bmiHeader.biCompression = biRGB

	hbm, _, _ := procCreateDIBSection.Call(
		hdcScreen,
		uintptr(unsafe.Pointer(&bi)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if hbm == 0 || bits == nil {
		return nil, errors.New("CreateDIBSection 失败")
	}
	defer procDeleteObjectIcon.Call(hbm)

	old, _, _ := procSelectObjectIcon.Call(hdcMem, hbm)
	if old != 0 {
		defer procSelectObjectIcon.Call(hdcMem, old)
	}

	// 透明底：DIB 默认可能未清零，显式清零。
	pixCount := size * size
	slice := unsafe.Slice((*byte)(bits), pixCount*4)
	for i := range slice {
		slice[i] = 0
	}

	r, _, callErr := procDrawIconEx.Call(
		hdcMem,
		0, 0,
		uintptr(hIcon),
		uintptr(size), uintptr(size),
		0, 0,
		diNormal,
	)
	if r == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return nil, callErr
		}
		return nil, errors.New("DrawIconEx 失败")
	}

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			i := (y*size + x) * 4
			// Windows DIB: BGRA
			b := slice[i+0]
			g := slice[i+1]
			r8 := slice[i+2]
			a := slice[i+3]
			off := img.PixOffset(x, y)
			img.Pix[off+0] = r8
			img.Pix[off+1] = g
			img.Pix[off+2] = b
			img.Pix[off+3] = a
		}
	}
	return img, nil
}

// resolveStartupExePath 从启动命令中解析可执行文件路径。
func resolveStartupExePath(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	// "C:\path\app.exe" args
	if strings.HasPrefix(command, `"`) {
		end := strings.Index(command[1:], `"`)
		if end >= 0 {
			return command[1 : 1+end]
		}
	}
	// 无引号：贪心拼接直到 Stat 命中（处理 Program Files 空格）。
	parts := splitCommandArgs(command)
	if len(parts) == 0 {
		return ""
	}
	candidate := parts[0]
	if fileExists(candidate) {
		return candidate
	}
	for i := 1; i < len(parts); i++ {
		candidate = candidate + " " + parts[i]
		if fileExists(candidate) {
			return candidate
		}
	}
	return parts[0]
}

func splitCommandArgs(command string) []string {
	var parts []string
	var b strings.Builder
	inQuote := false
	for _, r := range command {
		switch {
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t') && !inQuote:
			if b.Len() > 0 {
				parts = append(parts, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		parts = append(parts, b.String())
	}
	return parts
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
