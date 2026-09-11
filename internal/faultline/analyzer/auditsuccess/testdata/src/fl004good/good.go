package fl004good

import (
	"errors"
	"fmt"
	"log/slog"
)

var errAudit = errors.New("audit failed")

type unrelatedLogger struct{}

func (unrelatedLogger) Warn(string) {}

func failClosed() error {
	slog.Warn("audit: hard fail")
	return errAudit
}

func wrappedFailClosed() error {
	slog.Warn("audit: hard fail")
	return fmt.Errorf("audit: %w", errAudit)
}

func interveningFailClosed(flag bool) error {
	slog.Warn("audit: hard fail")
	if flag {
		return errAudit
	}
	return nil
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

func unrelatedWarn(logger unrelatedLogger) error {
	logger.Warn("not an audit sink")
	return nil
}

func boxedZero() (any, error) {
	slog.Warn("audit: boxed value is non-nil")
	return 0, nil
}

func boxedFalse() (any, error) {
	slog.Warn("audit: boxed value is non-nil")
	return false, nil
}

func boxedEmptyString() (any, error) {
	slog.Warn("audit: boxed value is non-nil")
	return "", nil
}

func suppressed() error {
	slog.Warn("audit: accepted nonfatal") // faultline:warn-nonfatal
	return nil
}

func precedingLineSuppressed() error {
	// faultline:warn-nonfatal
	slog.Warn("audit: accepted nonfatal")
	return nil
}

func warningAfterSuccess() error {
	return nil
	slog.Warn("audit: unreachable soft fail")
	return errAudit
}
