//go:build !windows

package main

import "errors"

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

func (s *PortService) ListPorts() (PortListResult, error) {
	return PortListResult{}, errors.New("PortCheck MVP 目前只支持 Windows")
}

func (s *PortService) KillProcess(pid uint32) (KillProcessResult, error) {
	return KillProcessResult{PID: pid}, errors.New("PortCheck MVP 目前只支持 Windows")
}
