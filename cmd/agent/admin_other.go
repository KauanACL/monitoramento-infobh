//go:build !windows

package main

func requireServiceAdmin() error {
	return nil
}
