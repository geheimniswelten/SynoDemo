//go:build !linux

package platform

import "errors"

var errAlreadyRunning = errors.New("package service is already running")

func lockService() (func(), error) {
	return nil, errors.New("the DSM package service requires Linux")
}
