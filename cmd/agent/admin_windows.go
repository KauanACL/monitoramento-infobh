//go:build windows

package main

import (
	"errors"

	"golang.org/x/sys/windows"
)

func requireServiceAdmin() error {
	admin, err := runningAsAdmin()
	if err != nil || !admin {
		return errors.New("para instalar o servico, abra o Prompt/PowerShell como Administrador e rode o comando novamente")
	}
	return nil
}

func runningAsAdmin() (bool, error) {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false, err
	}
	defer windows.FreeSid(sid)
	return windows.Token(0).IsMember(sid)
}
