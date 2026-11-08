//go:build !windows

package main

import "errors"

// FileLockEntry / FileLockResult / FileLockService 与 Windows 侧保持结构一致。
type FileLockEntry struct {
	PID       uint32 `json:"pid"`
	Process   string `json:"process"`
	ExePath   string `json:"exePath"`
	AppType   string `json:"appType"`
	StartTime int64  `json:"startTime"`
}

type FileLockResult struct {
	Entries  []FileLockEntry `json:"entries"`
	Scanned  int             `json:"scanned"`
	Warnings []string        `json:"warnings"`
}

type FileLockService struct{}

func (s *FileLockService) FindLockers(path string) (FileLockResult, error) {
	return FileLockResult{}, errors.New("PortCheck MVP 目前只支持 Windows")
}
