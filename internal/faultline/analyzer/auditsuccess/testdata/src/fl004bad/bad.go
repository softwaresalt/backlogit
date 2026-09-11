package fl004bad

import (
	"log/slog"

	"audit"
)

type recorder struct{}

func (recorder) AuditWarn(string) {}

func packageWarn() error {
	slog.Warn("audit: soft fail") // want "FL004"
	return nil
}

func loggerWarn(logger *slog.Logger) error {
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
