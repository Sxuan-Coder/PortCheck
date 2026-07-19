//go:build !windows

package main

import "errors"

// ListServices 非 Windows 占位。
func (s *ServicesService) ListServices() ([]ServiceEntry, error) {
	return nil, errors.New("服务枚举目前只支持 Windows")
}

// StopService 非 Windows 占位。
func (s *ServicesService) StopService(name string) (ServiceOpResult, error) {
	return ServiceOpResult{Name: name, Action: "stop"}, errors.New("服务操作目前只支持 Windows")
}

// StartService 非 Windows 占位。
func (s *ServicesService) StartService(name string) (ServiceOpResult, error) {
	return ServiceOpResult{Name: name, Action: "start"}, errors.New("服务操作目前只支持 Windows")
}

// ListStartup 非 Windows 占位。
func (s *StartupService) ListStartup() ([]StartupEntry, error) {
	return nil, errors.New("启动项枚举目前只支持 Windows")
}

// DeleteStartup 非 Windows 占位。
func (s *StartupService) DeleteStartup(name, location string) (StartupOpResult, error) {
	return StartupOpResult{Name: name, Action: "delete"}, errors.New("启动项操作目前只支持 Windows")
}

// DisableStartup 非 Windows 占位。
func (s *StartupService) DisableStartup(name, location string) (StartupOpResult, error) {
	return StartupOpResult{Name: name, Action: "disable"}, errors.New("启动项操作目前只支持 Windows")
}

// EnableStartup 非 Windows 占位。
func (s *StartupService) EnableStartup(name, location string) (StartupOpResult, error) {
	return StartupOpResult{Name: name, Action: "enable"}, errors.New("启动项操作目前只支持 Windows")
}
