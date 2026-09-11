package fl004bad

import (
	"errors"
	"log/slog"

	"audit"
)

var errAudit = errors.New("audit failed")

type recorder struct{}

func (recorder) AuditWarn(string) {}

type compositeResult struct {
	count int
}

type promotedPointerLogger struct {
	*slog.Logger
}

type promotedValueLogger struct {
	slog.Logger
}

func packageWarn() error {
	slog.Warn("audit: soft fail") // want "FL004"
	return nil
}

func loggerWarn(logger *slog.Logger) error {
	logger.Warn("audit: soft fail") // want "FL004"
	return nil
}

func loggerValueWarn(logger slog.Logger) error {
	logger.Warn("audit: soft fail") // want "FL004"
	return nil
}

func promotedPointerWarn(logger promotedPointerLogger) error {
	logger.Warn("audit: soft fail") // want "FL004"
	return nil
}

func promotedValueWarn(logger promotedValueLogger) error {
	logger.Warn("audit: soft fail") // want "FL004"
	return nil
}

func auditWarn() error {
	audit.Warn("audit: soft fail") // want "FL004"
	return nil
}

func namedAuditWarn(logger recorder) error {
	logger.AuditWarn("audit: soft fail") // want "FL004"
	return nil
}

func multiResultZeroValues() (int, compositeResult, *compositeResult, error) {
	slog.Warn("audit: soft fail") // want "FL004"
	return 0, compositeResult{}, nil, nil
}

func boxedNil() (any, error) {
	slog.Warn("audit: soft fail") // want "FL004"
	return nil, nil
}

func multipleWarnings() error {
	slog.Warn("audit: first soft fail")  // want "FL004"
	slog.Warn("audit: second soft fail") // want "FL004"
	return nil
}

func warningOrder(flag bool) error {
	slog.Warn("audit: cleared by intervening error")
	if flag {
		return errAudit
	}
	slog.Warn("audit: later soft fail") // want "FL004"
	return nil
}

func trailingSuppressionOwnsImmediateStatement() error {
//line bad.go:200
	// want +1 "FL004"
	slog.Warn("audit: not suppressed")
//line bad.go:201
	slog.Warn("audit: accepted nonfatal") // faultline:warn-nonfatal
	return nil
}

func precedingSuppressionOwnsImmediateStatement() error {
//line bad.go:300
	// faultline:warn-nonfatal
//line bad.go:300
	// want +1 "FL004"
//line bad.go:301
	slog.Warn("audit: accepted nonfatal")
//line bad.go:301
	slog.Warn("audit: not suppressed")
	return nil
}

func nearMissSuppression() error {
	// want +1 "FL004"
	slog.Warn("audit: soft fail") // faultline:warn-nonfatal because this is safe
	return nil
}

func nonDedicatedPrecedingSuppression() error {
	accepted := true              // faultline:warn-nonfatal
	slog.Warn("audit: soft fail") // want "FL004"
	_ = accepted
	return nil
}

func nonAdjacentPrecedingSuppression() error {
	// faultline:warn-nonfatal

	slog.Warn("audit: soft fail") // want "FL004"
	return nil
}
