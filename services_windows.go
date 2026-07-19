package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// protectedServiceNames 禁止停止的关键服务名单（避免系统异常）。
var protectedServiceNames = map[string]struct{}{
	"RpcSs":            {},
	"DcomLaunch":       {},
	"LSM":              {},
	"SamSs":            {},
	"Schedule":         {},
	"WinDefend":        {},
	"EventLog":         {},
	"PlugPlay":         {},
	"Power":            {},
	"CryptSvc":         {},
	"BFE":              {},
	"mpssvc":           {},
	"WdNisSvc":         {},
	"Winmgmt":          {},
	"ProfSvc":          {},
	"UserManager":      {},
	"CoreMessagingRegistrar": {},
	"SystemEventsBroker":     {},
	"StateRepository":        {},
	"BrokerInfrastructure":   {},
	"camsvc":                 {},
	"AppXSvc":                {},
}

// ListServices 以只读方式枚举所有 Windows 服务。
// 仅需 SC_MANAGER_ENUMERATE_SERVICE 权限，标准用户即可调用；
// 单次 EnumServicesStatusEx 同时拿到 名称/显示名/状态/类型/进程ID，无需逐个 OpenService。
func (s *ServicesService) ListServices() ([]ServiceEntry, error) {
	h, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT|windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil {
		return nil, err
	}
	defer windows.CloseServiceHandle(h)

	var needed, returned uint32
	var buf []byte
	for {
		var servicesPtr *byte
		if len(buf) > 0 {
			servicesPtr = &buf[0]
		}
		err := windows.EnumServicesStatusEx(
			h, windows.SC_STATUS_PROCESS_INFO,
			windows.SERVICE_TYPE_ALL, windows.SERVICE_STATE_ALL,
			servicesPtr, uint32(len(buf)),
			&needed, &returned, nil, nil,
		)
		if err == nil {
			break
		}
		if err != syscall.ERROR_MORE_DATA {
			return nil, err
		}
		buf = make([]byte, needed) // 按系统要求的字节数重新分配后再次枚举
	}

	if returned == 0 || len(buf) == 0 {
		return []ServiceEntry{}, nil
	}

	entries := unsafe.Slice((*windows.ENUM_SERVICE_STATUS_PROCESS)(unsafe.Pointer(&buf[0])), returned)
	out := make([]ServiceEntry, 0, returned)
	for i := uint32(0); i < returned; i++ {
		e := entries[i]
		out = append(out, ServiceEntry{
			Name:        windows.UTF16PtrToString(e.ServiceName),
			DisplayName: windows.UTF16PtrToString(e.DisplayName),
			State:       serviceNameState(e.ServiceStatusProcess.CurrentState),
			StartType:   serviceTypeLabel(e.ServiceStatusProcess.ServiceType),
		})
	}
	return out, nil
}

// StopService 停止指定服务；权限不足时自动 UAC 提权重试。
func (s *ServicesService) StopService(name string) (ServiceOpResult, error) {
	if name == "" {
		return ServiceOpResult{}, fmt.Errorf("服务名不能为空")
	}
	if err := ensureServiceStopAllowed(name); err != nil {
		return ServiceOpResult{Name: name, Action: "stop"}, err
	}
	err := tryDirectOrElevate(func() error {
		return stopServiceDirect(name)
	}, "svc-stop", name, "")
	if err != nil {
		return ServiceOpResult{Name: name, Action: "stop"}, err
	}
	return ServiceOpResult{
		Name:    name,
		Action:  "stop",
		Message: fmt.Sprintf("已发送停止请求：%s", name),
	}, nil
}

// StartService 启动指定服务；权限不足时自动 UAC 提权重试。
func (s *ServicesService) StartService(name string) (ServiceOpResult, error) {
	if name == "" {
		return ServiceOpResult{}, fmt.Errorf("服务名不能为空")
	}
	err := tryDirectOrElevate(func() error {
		return startServiceDirect(name)
	}, "svc-start", name, "")
	if err != nil {
		return ServiceOpResult{Name: name, Action: "start"}, err
	}
	return ServiceOpResult{
		Name:    name,
		Action:  "start",
		Message: fmt.Sprintf("已启动服务：%s", name),
	}, nil
}

// ensureServiceStopAllowed 校验服务是否允许停止。
func ensureServiceStopAllowed(name string) error {
	if _, ok := protectedServiceNames[name]; ok {
		return fmt.Errorf("服务 %s 属于关键系统服务，禁止停止", name)
	}
	return nil
}

// stopServiceDirect 进程内直接停止服务（供 Service 方法与提权子进程共用）。
func stopServiceDirect(name string) error {
	if err := ensureServiceStopAllowed(name); err != nil {
		return err
	}
	mgr, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(mgr)

	svcName, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	svc, err := windows.OpenService(mgr, svcName, windows.SERVICE_STOP|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(svc)

	var status windows.SERVICE_STATUS
	err = windows.ControlService(svc, windows.SERVICE_CONTROL_STOP, &status)
	if err != nil {
		// 已停止视为成功。
		if errno, ok := err.(syscall.Errno); ok && errno == windows.ERROR_SERVICE_NOT_ACTIVE {
			return nil
		}
		return err
	}
	return nil
}

// startServiceDirect 进程内直接启动服务。
func startServiceDirect(name string) error {
	mgr, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(mgr)

	svcName, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	svc, err := windows.OpenService(mgr, svcName, windows.SERVICE_START|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(svc)

	err = windows.StartService(svc, 0, nil)
	if err != nil {
		// 已运行视为成功。
		if errno, ok := err.(syscall.Errno); ok && errno == windows.ERROR_SERVICE_ALREADY_RUNNING {
			return nil
		}
		return err
	}
	return nil
}

// serviceNameState 把 Win32 服务状态码映射为中文文案。
func serviceNameState(state uint32) string {
	switch state {
	case 1:
		return "已停止"
	case 4:
		return "运行中"
	case 7:
		return "已暂停"
	case 2:
		return "启动中"
	case 3:
		return "停止中"
	case 5:
		return "继续中"
	case 6:
		return "暂停中"
	default:
		return "未知"
	}
}

// serviceTypeLabel 把 Win32 ServiceType 位标志映射为简短类型文案。
func serviceTypeLabel(t uint32) string {
	switch {
	case t&0x1 != 0, t&0x2 != 0:
		return "驱动"
	case t&windows.SERVICE_WIN32_OWN_PROCESS != 0 && t&windows.SERVICE_WIN32_SHARE_PROCESS != 0:
		return "独立/共享进程"
	case t&windows.SERVICE_WIN32_SHARE_PROCESS != 0:
		return "共享进程"
	case t&windows.SERVICE_WIN32_OWN_PROCESS != 0:
		return "独立进程"
	default:
		return "其他"
	}
}
