package main

import (
	_ "embed"
	"fmt"
	"io"
)

// thirdPartyLicenses holds the license texts of the third-party software in
// the binary, so that they travel with it also after "go install".
// scripts/third-party-licenses.sh writes the file; TestLicensesListsEveryModule
// fails when a module is missing from it.
//
//go:embed third_party_licenses.txt
var thirdPartyLicenses string

// runLicenses prints the license texts of the third-party software in the
// binary.
func runLicenses(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "concourse-lint: licenses takes no arguments, got %q\n", args[0])
		return exitError
	}
	if _, err := io.WriteString(stdout, thirdPartyLicenses); err != nil {
		fmt.Fprintf(stderr, "concourse-lint: %v\n", err)
		return exitError
	}
	return exitOK
}
