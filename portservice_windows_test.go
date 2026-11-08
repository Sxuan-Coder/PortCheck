package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestTCPStateName(t *testing.T) {
	if got := tcpStateName(2); got != "LISTENING" {
		t.Fatalf("tcpStateName(2) = %q", got)
	}
	if got := tcpStateName(5); got != "ESTABLISHED" {
		t.Fatalf("tcpStateName(5) = %q", got)
	}
}

func TestValidateKillPIDProtectsSystemAndSelf(t *testing.T) {
	protected := []uint32{0, 4, uint32(os.Getpid())}
	for _, pid := range protected {
		if err := validateKillPID(pid); err == nil {
			t.Fatalf("validateKillPID(%d) should reject protected pid", pid)
		}
	}
}

func TestListTCPPortsFindsCurrentProcessListener(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	rows, err := listTCPPorts()
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range rows {
		if row.Protocol == "TCP" && row.LocalPort == port && row.PID == uint32(os.Getpid()) {
			return
		}
	}
	t.Fatalf("current TCP listener on port %d was not found", port)
}

func TestListTCPPortsFindsCurrentProcessIPv6Listener(t *testing.T) {
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback is not available: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	rows, err := listTCPPorts()
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range rows {
		if row.Protocol == "TCP" && row.LocalAddr == "::1" && row.LocalPort == port && row.PID == uint32(os.Getpid()) {
			return
		}
	}
	t.Fatalf("current TCP IPv6 listener on port %d was not found", port)
}

func TestListUDPPortsFindsCurrentProcessListener(t *testing.T) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	port := conn.LocalAddr().(*net.UDPAddr).Port
	rows, err := listUDPPorts()
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range rows {
		if row.Protocol == "UDP" && row.LocalPort == port && row.PID == uint32(os.Getpid()) {
			return
		}
	}
	t.Fatalf("current UDP listener on port %d was not found", port)
}

func TestListUDPPortsFindsCurrentProcessIPv6Listener(t *testing.T) {
	conn, err := net.ListenPacket("udp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback is not available: %v", err)
	}
	defer conn.Close()

	port := conn.LocalAddr().(*net.UDPAddr).Port
	rows, err := listUDPPorts()
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range rows {
		if row.Protocol == "UDP" && row.LocalAddr == "::1" && row.LocalPort == port && row.PID == uint32(os.Getpid()) {
			return
		}
	}
	t.Fatalf("current UDP IPv6 listener on port %d was not found", port)
}

func TestKillProcessTerminatesChildProcess(t *testing.T) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Start-Sleep -Seconds 30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	service := &PortService{}
	if _, err := service.KillProcess(uint32(cmd.Process.Pid)); err != nil {
		_ = cmd.Process.Kill()
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("child process was not terminated")
	}
}

func TestKillProcessTerminatesListeningChildProcess(t *testing.T) {
	port := unusedTCPPort(t)
	script := fmt.Sprintf(
		"$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Parse('127.0.0.1'), %d); "+
			"$listener.Start(); Start-Sleep -Seconds 30; $listener.Stop()",
		port,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	if !waitForTCPPortPID(port, uint32(cmd.Process.Pid), 5*time.Second) {
		t.Fatalf("helper process did not listen on port %d", port)
	}

	service := &PortService{}
	if _, err := service.KillProcess(uint32(cmd.Process.Pid)); err != nil {
		t.Fatal(err)
	}

	// taskkill 返回时终止流程可能尚未完成，先确定性等待进程退出，
	// 再断言端口不再被活进程监听（死进程残留的 TCB 行不算占用）。
	if !waitForProcessExit(uint32(cmd.Process.Pid), 10*time.Second) {
		t.Fatalf("killed pid %d did not exit in time", cmd.Process.Pid)
	}
	if owner := findLiveTCPPortOwner(port, 2*time.Second); owner != nil {
		t.Fatalf("port %d is still owned by live pid %d after killing %d", port, owner.PID, cmd.Process.Pid)
	}
}

// waitForProcessExit 轮询直到 PID 不再对应运行中的进程。
func waitForProcessExit(pid uint32, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !testProcessAlive(pid) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// findLiveTCPPortOwner 轮询端口表，返回该端口仍被活进程监听时命中的行；超时未占用返回 nil。
// 死进程残留的 LISTENING 行不算占用，避免 TCB 清理延迟导致的间歇性失败。
func findLiveTCPPortOwner(port int, timeout time.Duration) *PortEntry {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rows, err := listTCPPorts()
		if err == nil {
			for i := range rows {
				row := &rows[i]
				if row.LocalPort == port && row.State == "LISTENING" && testProcessAlive(row.PID) {
					return row
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// testProcessAlive 判断 PID 是否对应一个仍在运行的进程（已退出但未清理的不算）。
func testProcessAlive(pid uint32) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false
	}
	return status == uint32(windows.WAIT_TIMEOUT)
}

func unusedTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

func waitForTCPPortPID(port int, pid uint32, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rows, err := listTCPPorts()
		if err == nil {
			for _, row := range rows {
				if row.LocalPort == port && row.PID == pid && row.State == "LISTENING" {
					return true
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func TestIPv4AndPortEncodingRoundTrip(t *testing.T) {
	for _, addr := range []string{"127.0.0.1", "192.168.1.5", "0.0.0.0", "255.255.255.255"} {
		enc, err := ipv4ToDWORD(addr)
		if err != nil {
			t.Fatalf("ipv4ToDWORD(%q): %v", addr, err)
		}
		if got := ipv4FromDWORD(enc); got != addr {
			t.Fatalf("round trip %q -> %q", addr, got)
		}
	}
	if _, err := ipv4ToDWORD("::1"); err == nil {
		t.Fatal("IPv6 地址应被拒绝")
	}
	for _, port := range []int{0, 80, 443, 8080, 65535} {
		if got := portFromDWORD(portToDWORD(port)); got != port {
			t.Fatalf("port round trip %d -> %d", port, got)
		}
	}
}

func TestCloseConnPackUnpack(t *testing.T) {
	req := CloseConnRequest{
		IPVersion: "v4", LocalAddr: "192.168.1.5", LocalPort: 52341,
		RemoteAddr: "93.184.216.34", RemotePort: 443, PID: 4242,
	}
	back, err := unpackCloseConnRequest(packCloseConnRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if back != req {
		t.Fatalf("round trip mismatch: %+v vs %+v", back, req)
	}
	if _, err := unpackCloseConnRequest("bad-format"); err == nil {
		t.Fatal("非法格式应报错")
	}
}
