//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package core_test

import "errors"

func holdAdvisoryLock(string) (func(), error) {
	return nil, errors.New("advisory test lock unsupported on this platform")
}
