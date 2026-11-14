//go:build linux

package platform

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

var errAlreadyRunning = errors.New("package service is already running")

func lockService() (func(), error) {
	file, err := os.OpenFile(filepath.Join(DataDir(), ".service-lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errAlreadyRunning
		}
		return nil, err
	}
	return func() { _ = file.Close() }, nil
}
