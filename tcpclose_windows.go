//go:build windows

package main

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procSetTcpEntry = iphlpapi.NewProc("SetTcpEntry")

// mibTCPRow 对应 Win32 MIB_TCPROW（SetTcpEntry 参数，20 字节，无 PID 字段）。
type mibTCPRow struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
}

// tcpStateDeleteTCB 是 SetTcpEntry 唯一允许写入的目标状态。
const tcpStateDeleteTCB = 12

// CloseConnRequest 描述要断开的一条 IPv4 TCP 连接。
// PID 参与行匹配：防止四元组在确认/提权等待期间被其他进程复用后关错连接。
type CloseConnRequest struct {
	IPVersion  string `json:"ipVersion"` // 仅支持 "v4"
	LocalAddr  string `json:"localAddr"`
	LocalPort  int    `json:"localPort"`
	RemoteAddr string `json:"remoteAddr"`
	RemotePort int    `json:"remotePort"`
	PID        uint32 `json:"pid"`
}

// CloseConnResult 是断开连接操作的结果。
type CloseConnResult struct {
	Message string `json:"message"`
}

// CloseConnection 断开一条 IPv4 TCP 连接，不结束进程。
// 先直接尝试 SetTcpEntry；权限不足（错误码 5/317）时走提权协议重试，
// 取消 UAC 等同未操作，不改变界面状态。
func (s *PortService) CloseConnection(req CloseConnRequest) (CloseConnResult, error) {
	if req.IPVersion != "v4" {
		return CloseConnResult{}, errors.New("当前仅支持断开 IPv4 TCP 连接")
	}
	if req.PID == 0 || req.PID == 4 {
		return CloseConnResult{}, errUnsupportedPID
	}

	err := closeTCPRowDirect(req.LocalAddr, req.LocalPort, req.RemoteAddr, req.RemotePort, req.PID)
	if err == nil {
		return CloseConnResult{Message: fmt.Sprintf("已断开 %s:%d ↔ %s:%d 的连接", req.LocalAddr, req.LocalPort, req.RemoteAddr, req.RemotePort)}, nil
	}
	var errno syscall.Errno
	if !(errors.As(err, &errno) && (errno == winErrorAccessDenied || errno == winErrorNotElevated)) {
		return CloseConnResult{}, classifyActionError(err)
	}

	// 提权重试：打包行信息，提权子进程重新匹配复核后执行。
	if elevErr := runElevatedOp("tcp-close", packCloseConnRequest(req), ""); elevErr != nil {
		return CloseConnResult{}, elevErr
	}
	return CloseConnResult{Message: fmt.Sprintf("已断开 %s:%d ↔ %s:%d 的连接", req.LocalAddr, req.LocalPort, req.RemoteAddr, req.RemotePort)}, nil
}

// closeTCPRowDirect 在当前 TCPv4 表中按四元组+PID 匹配行，校验状态后置 DELETE_TCB。
func closeTCPRowDirect(laddr string, lport int, raddr string, rport int, pid uint32) error {
	row, err := findTCP4Row(laddr, lport, raddr, rport, pid)
	if err != nil {
		return err
	}
	del := mibTCPRow{
		State:      tcpStateDeleteTCB,
		LocalAddr:  row.LocalAddr,
		LocalPort:  row.LocalPort,
		RemoteAddr: row.RemoteAddr,
		RemotePort: row.RemotePort,
	}
	r1, _, _ := procSetTcpEntry.Call(uintptr(unsafe.Pointer(&del)))
	if r1 != 0 {
		return windows.Errno(r1)
	}
	return nil
}

// findTCP4Row 按（网络字节序编码后的）四元组与 PID 查找原始行；
// 仅接受可断开状态（监听与已关闭的套接字不属于"断开连接"语义）。
func findTCP4Row(laddr string, lport int, raddr string, rport int, pid uint32) (mibTCPRowOwnerPID, error) {
	local, err := ipv4ToDWORD(laddr)
	if err != nil {
		return mibTCPRowOwnerPID{}, err
	}
	remote, err := ipv4ToDWORD(raddr)
	if err != nil {
		return mibTCPRowOwnerPID{}, err
	}
	wantLocal, wantRemote := portToDWORD(lport), portToDWORD(rport)

	buf, err := queryTable(procGetExtendedTCPTable, afInet, tcpTableOwnerPIDAll)
	if err != nil {
		return mibTCPRowOwnerPID{}, fmt.Errorf("读取 TCP 连接表失败：%w", err)
	}
	count := *(*uint32)(unsafe.Pointer(&buf[0]))
	rowSize := unsafe.Sizeof(mibTCPRowOwnerPID{})
	for i := uint32(0); i < count; i++ {
		offset := uintptr(4) + uintptr(i)*rowSize
		if int(offset)+int(rowSize) > len(buf) {
			break
		}
		row := (*mibTCPRowOwnerPID)(unsafe.Pointer(&buf[offset]))
		if row.OwningPID != pid || row.LocalAddr != local || row.LocalPort != wantLocal ||
			row.RemoteAddr != remote || row.RemotePort != wantRemote {
			continue
		}
		if row.State < 3 || row.State > tcpStateDeleteTCB {
			return mibTCPRowOwnerPID{}, errors.New("该连接处于监听或已关闭状态，不支持断开")
		}
		return *row, nil
	}
	return mibTCPRowOwnerPID{}, errors.New("连接已关闭或状态已变化，请刷新后重试")
}

// packCloseConnRequest 把请求打包进提权协议的 --name 字段：
// "laddr:lport,raddr:rport,pid"（IPv4 无冒号/逗号歧义）。
func packCloseConnRequest(req CloseConnRequest) string {
	return fmt.Sprintf("%s:%d,%s:%d,%d", req.LocalAddr, req.LocalPort, req.RemoteAddr, req.RemotePort, req.PID)
}

// unpackCloseConnRequest 解析提权协议传回的行信息。
func unpackCloseConnRequest(packed string) (CloseConnRequest, error) {
	parts := strings.Split(packed, ",")
	if len(parts) != 3 {
		return CloseConnRequest{}, fmt.Errorf("断开连接参数格式错误：%q", packed)
	}
	local := strings.SplitN(parts[0], ":", 2)
	remote := strings.SplitN(parts[1], ":", 2)
	if len(local) != 2 || len(remote) != 2 {
		return CloseConnRequest{}, fmt.Errorf("断开连接参数格式错误：%q", packed)
	}
	var pid uint32
	var lport, rport int
	if _, err := fmt.Sscanf(parts[2], "%d", &pid); err != nil {
		return CloseConnRequest{}, fmt.Errorf("断开连接 PID 解析失败：%w", err)
	}
	if _, err := fmt.Sscanf(local[1], "%d", &lport); err != nil {
		return CloseConnRequest{}, fmt.Errorf("断开连接端口解析失败：%w", err)
	}
	if _, err := fmt.Sscanf(remote[1], "%d", &rport); err != nil {
		return CloseConnRequest{}, fmt.Errorf("断开连接端口解析失败：%w", err)
	}
	return CloseConnRequest{
		IPVersion:  "v4",
		LocalAddr:  local[0],
		LocalPort:  lport,
		RemoteAddr: remote[0],
		RemotePort: rport,
		PID:        pid,
	}, nil
}

// closeTCPDirect 是提权子进程入口：解析打包行信息后重新匹配并断开。
func closeTCPDirect(packed string) error {
	req, err := unpackCloseConnRequest(packed)
	if err != nil {
		return err
	}
	return closeTCPRowDirect(req.LocalAddr, req.LocalPort, req.RemoteAddr, req.RemotePort, req.PID)
}

// ipv4ToDWORD 把点分 IPv4 编码为端口表中行的字节序（首八位组在最低字节）。
func ipv4ToDWORD(addr string) (uint32, error) {
	ip := net.ParseIP(addr).To4()
	if ip == nil {
		return 0, fmt.Errorf("无效的 IPv4 地址：%s", addr)
	}
	return uint32(ip[0]) | uint32(ip[1])<<8 | uint32(ip[2])<<16 | uint32(ip[3])<<24, nil
}

// portToDWORD 把端口号编码为行内网络字节序（高字节在低地址）。
func portToDWORD(port int) uint32 {
	p := uint16(port)
	return uint32(p>>8) | uint32(p&0xFF)<<8
}
