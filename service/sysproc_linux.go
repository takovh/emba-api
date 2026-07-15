package service

import "syscall"

func setPG() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
