//go:build windows

package main

import (
	"errors"
	"syscall"
)

// 操作类 sentinel 错误：让提权链路保留错误身份（而非纯文案），
// 供 classifyActionError 与前端提示分类识别。
var (
	// errElevateCancelled 用户在 UAC 弹窗选择取消，等同未执行任何操作。
	errElevateCancelled = errors.New("已取消管理员授权")
	// errElevateTimeout 等待提权子进程超出时限。
	errElevateTimeout = errors.New("提权操作超时")
	// errActionAccessDenied 直接执行与提权重试都因权限失败后的统一文案。
	errActionAccessDenied = errors.New("权限不足：该操作需要管理员权限，请以管理员身份运行 PortCheck 后重试")
)

// winErrorNotElevated 对应 SetTcpEntry 在非提权进程中的专属错误码 317，
// 文档明确区分于通用 ERROR_ACCESS_DENIED，两者都表示需要提权。
const winErrorNotElevated = syscall.Errno(317)

// classifyActionError 把操作类错误归一为面向用户的可操作文案。
// 已带清晰文案的错误（sentinel、提权子进程回传消息）原样透传；
// 裸 Errno 按错误码映射，避免调用方各自维护文案。
func classifyActionError(err error) error {
	if err == nil {
		return nil
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case winErrorAccessDenied, winErrorNotElevated:
			return errActionAccessDenied
		}
	}
	return err
}
