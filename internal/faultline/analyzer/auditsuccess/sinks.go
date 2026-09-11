package auditsuccess

type auditWarningSinkKind uint8

const (
	packageFunctionSink auditWarningSinkKind = iota
	loggerMethodSink
	selectorNameSink
)

type auditWarningSink struct {
	kind            auditWarningSinkKind
	selector        string
	packagePath     string
	receiverPackage string
	receiverName    string
}

// auditWarningSinks is the declaration-driven allowlist for FL004.
var auditWarningSinks = [...]auditWarningSink{
	{
		kind:        packageFunctionSink,
		selector:    "Warn",
		packagePath: "log/slog",
	},
	{
		kind:            loggerMethodSink,
		selector:        "Warn",
		receiverPackage: "log/slog",
		receiverName:    "Logger",
	},
	{
		kind:        packageFunctionSink,
		selector:    "Warn",
		packagePath: "audit",
	},
	{
		kind:     selectorNameSink,
		selector: "AuditWarn",
	},
}
