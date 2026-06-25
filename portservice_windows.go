package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	afInet              = 2
	tcpTableOwnerPIDAll = 5
	udpTableOwnerPID    = 1
)

var (
	iphlpapi                = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTCPTable = iphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUDPTable = iphlpapi.NewProc("GetExtendedUdpTable")
	errUnsupportedPID       = errors.New("系统进程或当前应用进程不允许结束")
	errTerminatePrivilege   = errors.New("结束进程失败，可能需要管理员权限或该进程不允许结束")
	errTerminateTimeout     = errors.New("已发送结束进程请求，但进程没有在预期时间内退出")
)

type PortService struct{}

type PortEntry struct {
	Protocol   string `json:"protocol"`
	LocalAddr  string `json:"localAddr"`
	LocalPort  int    `json:"localPort"`
	RemoteAddr string `json:"remoteAddr"`
	RemotePort int    `json:"remotePort"`
	State      string `json:"state"`
	PID        uint32 `json:"pid"`
	Process    string `json:"process"`
	Path       string `json:"path"`
}

type PortListResult struct {
	Ports          []PortEntry `json:"ports"`
	TCPCount       int         `json:"tcpCount"`
	UDPCount       int         `json:"udpCount"`
	ListeningCount int         `json:"listeningCount"`
	ProcessCount   int         `json:"processCount"`
	Warnings       []string    `json:"warnings"`
}

type KillProcessResult struct {
	PID     uint32 `json:"pid"`
	Message string `json:"message"`
}

type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	OwningPID  uint32
}

type mibUDPRowOwnerPID struct {
	LocalAddr uint32
	LocalPort uint32
	OwningPID uint32
}

func (s *PortService) ListPorts() (PortListResult, error) {
	tcpRows, tcpErr := listTCPPorts()
	udpRows, udpErr := listUDPPorts()

	result := PortListResult{
		Ports: append(tcpRows, udpRows...),
	}
	if tcpErr != nil {
		result.Warnings = append(result.Warnings, "TCP 端口读取失败："+tcpErr.Error())
	}
	if udpErr != nil {
		result.Warnings = append(result.Warnings, "UDP 端口读取失败："+udpErr.Error())
	}
	if tcpErr != nil && udpErr != nil {
		return result, fmt.Errorf("端口读取失败：%v；%v", tcpErr, udpErr)
	}

	processes := map[uint32]bool{}
	for i := range result.Ports {
		if result.Ports[i].Protocol == "TCP" {
			result.TCPCount++
			if result.Ports[i].State == "LISTENING" {
				result.ListeningCount++
			}
		} else {
			result.UDPCount++
			result.ListeningCount++
		}
		processes[result.Ports[i].PID] = true
	}
	result.ProcessCount = len(processes)

	sort.SliceStable(result.Ports, func(i, j int) bool {
		if result.Ports[i].LocalPort == result.Ports[j].LocalPort {
			if result.Ports[i].Protocol == result.Ports[j].Protocol {
				return result.Ports[i].PID < result.Ports[j].PID
			}
			return result.Ports[i].Protocol < result.Ports[j].Protocol
		}
		return result.Ports[i].LocalPort < result.Ports[j].LocalPort
	})

	return result, nil
}

func (s *PortService) KillProcess(pid uint32) (KillProcessResult, error) {
	if err := validateKillPID(pid); err != nil {
		return KillProcessResult{PID: pid}, err
	}

	if err := taskkillProcessTree(pid); err == nil {
		return KillProcessResult{
			PID:     pid,
			Message: fmt.Sprintf("已强制结束 PID %d 对应的进程树", pid),
		}, nil
	}

	if err := terminateProcess(pid); err != nil {
		return KillProcessResult{PID: pid}, err
	}

	return KillProcessResult{
		PID:     pid,
		Message: fmt.Sprintf("已结束 PID %d 对应的进程", pid),
	}, nil
}

func terminateProcess(pid uint32) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return errTerminatePrivilege
	}
	defer windows.CloseHandle(handle)

	if err := windows.TerminateProcess(handle, 1); err != nil {
		return errTerminatePrivilege
	}

	status, err := windows.WaitForSingleObject(handle, 5000)
	if err != nil {
		return errTerminatePrivilege
	}
	if status == uint32(windows.WAIT_TIMEOUT) {
		return errTerminateTimeout
	}
	if status != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("结束进程状态异常：%d", status)
	}

	return nil
}

func taskkillProcessTree(pid uint32) error {
	cmd := exec.Command("taskkill", "/PID", strconv.FormatUint(uint64(pid), 10), "/T", "/F")
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	return fmt.Errorf("结束进程失败，可能需要管理员权限或该进程不允许结束：%s", message)
}

func validateKillPID(pid uint32) error {
	if pid == 0 || pid == 4 || pid == uint32(os.Getpid()) {
		return errUnsupportedPID
	}
	return nil
}

func listTCPPorts() ([]PortEntry, error) {
	buf, err := queryTable(procGetExtendedTCPTable, tcpTableOwnerPIDAll)
	if err != nil {
		return nil, err
	}

	count := *(*uint32)(unsafe.Pointer(&buf[0]))
	rowSize := unsafe.Sizeof(mibTCPRowOwnerPID{})
	ports := make([]PortEntry, 0, count)
	processCache := map[uint32]processInfo{}

	for i := uint32(0); i < count; i++ {
		offset := uintptr(4) + uintptr(i)*rowSize
		row := (*mibTCPRowOwnerPID)(unsafe.Pointer(&buf[offset]))
		info := cachedProcessInfo(processCache, row.OwningPID)
		ports = append(ports, PortEntry{
			Protocol:   "TCP",
			LocalAddr:  ipv4FromDWORD(row.LocalAddr),
			LocalPort:  portFromDWORD(row.LocalPort),
			RemoteAddr: ipv4FromDWORD(row.RemoteAddr),
			RemotePort: portFromDWORD(row.RemotePort),
			State:      tcpStateName(row.State),
			PID:        row.OwningPID,
			Process:    info.name,
			Path:       info.path,
		})
	}

	return ports, nil
}

func listUDPPorts() ([]PortEntry, error) {
	buf, err := queryTable(procGetExtendedUDPTable, udpTableOwnerPID)
	if err != nil {
		return nil, err
	}

	count := *(*uint32)(unsafe.Pointer(&buf[0]))
	rowSize := unsafe.Sizeof(mibUDPRowOwnerPID{})
	ports := make([]PortEntry, 0, count)
	processCache := map[uint32]processInfo{}

	for i := uint32(0); i < count; i++ {
		offset := uintptr(4) + uintptr(i)*rowSize
		row := (*mibUDPRowOwnerPID)(unsafe.Pointer(&buf[offset]))
		info := cachedProcessInfo(processCache, row.OwningPID)
		ports = append(ports, PortEntry{
			Protocol:  "UDP",
			LocalAddr: ipv4FromDWORD(row.LocalAddr),
			LocalPort: portFromDWORD(row.LocalPort),
			State:     "-",
			PID:       row.OwningPID,
			Process:   info.name,
			Path:      info.path,
		})
	}

	return ports, nil
}

func queryTable(proc *windows.LazyProc, tableClass uint32) ([]byte, error) {
	var size uint32
	r1, _, err := proc.Call(0, uintptr(unsafe.Pointer(&size)), 0, afInet, uintptr(tableClass), 0)
	if r1 != uintptr(syscall.ERROR_INSUFFICIENT_BUFFER) && r1 != 0 {
		return nil, windows.Errno(r1)
	}
	if size == 0 {
		return nil, err
	}

	buf := make([]byte, size)
	r1, _, _ = proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, afInet, uintptr(tableClass), 0)
	if r1 != 0 {
		return nil, windows.Errno(r1)
	}
	return buf, nil
}

func portFromDWORD(port uint32) int {
	bytes := (*[4]byte)(unsafe.Pointer(&port))
	return int(binary.BigEndian.Uint16(bytes[:2]))
}

func ipv4FromDWORD(addr uint32) string {
	if addr == 0 {
		return "0.0.0.0"
	}
	bytes := (*[4]byte)(unsafe.Pointer(&addr))
	return net.IPv4(bytes[0], bytes[1], bytes[2], bytes[3]).String()
}

func tcpStateName(state uint32) string {
	switch state {
	case 1:
		return "CLOSED"
	case 2:
		return "LISTENING"
	case 3:
		return "SYN-SENT"
	case 4:
		return "SYN-RECEIVED"
	case 5:
		return "ESTABLISHED"
	case 6:
		return "FIN-WAIT-1"
	case 7:
		return "FIN-WAIT-2"
	case 8:
		return "CLOSE-WAIT"
	case 9:
		return "CLOSING"
	case 10:
		return "LAST-ACK"
	case 11:
		return "TIME-WAIT"
	case 12:
		return "DELETE-TCB"
	default:
		return fmt.Sprintf("UNKNOWN-%d", state)
	}
}

type processInfo struct {
	name string
	path string
}

func cachedProcessInfo(cache map[uint32]processInfo, pid uint32) processInfo {
	if info, ok := cache[pid]; ok {
		return info
	}
	info := queryProcessInfo(pid)
	cache[pid] = info
	return info
}

func queryProcessInfo(pid uint32) processInfo {
	if pid == 0 {
		return processInfo{name: "System Idle Process"}
	}
	if pid == 4 {
		return processInfo{name: "System"}
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return processInfo{name: fmt.Sprintf("PID %d", pid)}
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return processInfo{name: fmt.Sprintf("PID %d", pid)}
	}

	path := windows.UTF16ToString(buffer[:size])
	return processInfo{
		name: filepath.Base(path),
		path: path,
	}
}
