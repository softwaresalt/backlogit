package main

import (
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/scannerdiscipline"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	multichecker.Main(
		scannerdiscipline.Analyzer, // FL001
		// FL002: errwrap.Analyzer
		// FL003: failopen.Analyzer
		// FL004: auditsuccess.Analyzer
		// FL005: locktimeout.Analyzer
	)
}
