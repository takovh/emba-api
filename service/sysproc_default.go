//go:build !linux

package service

import "syscall"

func setPG() *syscall.SysProcAttr {
	return nil
}
