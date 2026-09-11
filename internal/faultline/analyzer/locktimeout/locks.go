package locktimeout

type timeoutSource struct {
	packagePath string
	function    string
}

// timeoutSources is the declaration-driven allowlist for FL005 timeout claims.
var timeoutSources = [...]timeoutSource{
	{
		packagePath: "context",
		function:    "WithTimeout",
	},
	{
		packagePath: "context",
		function:    "WithDeadline",
	},
}

type lockSink struct {
	selector        string
	receiverPackage string
	receiverName    string
	anyReceiver     bool
}

// uncancellableLockSinks is the declaration-driven allowlist for FL005 lock acquisitions.
var uncancellableLockSinks = [...]lockSink{
	{
		selector:    "Lock",
		anyReceiver: true,
	},
	{
		selector:        "RLock",
		receiverPackage: "sync",
		receiverName:    "RWMutex",
	},
	{
		selector:    "Acquire",
		anyReceiver: true,
	},
}
