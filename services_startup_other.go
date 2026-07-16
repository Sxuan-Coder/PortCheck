//go:build !windows

package main

import "errors"

// ListServices 非 Windows 占位。
func (s *ServicesService) ListServices() ([]ServiceEntry, error) {
	return nil, errors.New("服务枚举目前只支持 Windows")
}

// ListStartup 非 Windows 占位。
func (s *StartupService) ListStartup() ([]StartupEntry, error) {
	return nil, errors.New("启动项枚举目前只支持 Windows")
}
