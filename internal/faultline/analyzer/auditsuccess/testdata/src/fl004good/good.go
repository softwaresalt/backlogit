package fl004good

import (
	"errors"
	"log/slog"
)

var errAudit = errors.New("audit failed")

func failClosed() error {
	slog.Warn("audit: hard fail")
	return errAudit
}

func differentBlock(flag bool) error {
	if flag {
		slog.Warn("audit: soft fail")
	}
	return nil
}

func closure() error {
	func() {
		slog.Warn("audit: soft fail")
	}()
	return nil
}

func suppressed() error {
	slog.Warn("audit: accepted nonfatal") // faultline:warn-nonfatal
	return nil
}
