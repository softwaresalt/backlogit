package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/auditsuccess"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/errwrap"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/failopen"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/locktimeout"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/scannerdiscipline"
)

func main() {
	multichecker.Main(
		scannerdiscipline.Analyzer, // FL001
		errwrap.Analyzer,           // FL002
		failopen.Analyzer,          // FL003
		auditsuccess.Analyzer,      // FL004
		locktimeout.Analyzer,       // FL005
	)
}
