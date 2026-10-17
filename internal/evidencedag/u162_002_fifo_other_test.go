//go:build !unix

package evidencedag

import "errors"

func mkfifo(string) error { return errors.New("fifo unsupported") }
