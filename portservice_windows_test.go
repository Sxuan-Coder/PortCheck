package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
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

	if waitForTCPPortPID(port, uint32(cmd.Process.Pid), 2*time.Second) {
		t.Fatalf("port %d is still owned by pid %d", port, cmd.Process.Pid)
	}
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
