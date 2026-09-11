// Package scannerdiscipline declares the FL001 scanner-discipline analyzer.
package scannerdiscipline

import "golang.org/x/tools/go/analysis"

// Analyzer declares the FL001 scanner-discipline analyzer.
var Analyzer *analysis.Analyzer = &analysis.Analyzer{
	Name: "FL001scannerdiscipline",
	Doc:  "reports scanner loops that do not check scanner errors",
	Run:  run,
}

func run(_ *analysis.Pass) (any, error) {
	return nil, nil
}
