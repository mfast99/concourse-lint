package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLicenses(t *testing.T) {
	result := runCLI(t, "", "licenses")
	want, err := os.ReadFile("third_party_licenses.txt")
	if err != nil {
		t.Fatal(err)
	}
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("got exit %d, stderr %q, want exit 0 and no stderr", result.exitCode, result.stderr)
	}
	if result.stdout != string(want) {
		t.Errorf("stdout is not third_party_licenses.txt:\n%s", result.stdout)
	}
}

// TestLicensesListsEveryModule catches a dependency that was added or
// updated without running scripts/third-party-licenses.sh: every module
// that Go records in the binary must appear with its version.
func TestLicensesListsEveryModule(t *testing.T) {
	buildInfo, err := exec.Command("go", "version", "-m", binary).Output()
	if err != nil {
		t.Fatal(err)
	}
	licenses := runCLI(t, "", "licenses").stdout
	for line := range strings.Lines(string(buildInfo)) {
		fields := strings.Fields(line)
		// "dep <path> <version> <sum>" and, for a replaced module, the next
		// line "=> <path> <version> <sum>".
		if len(fields) < 3 || (fields[0] != "dep" && fields[0] != "=>") {
			continue
		}
		module := fields[1] + " " + fields[2]
		if !strings.Contains(licenses, module) {
			t.Errorf("licenses does not list %s: run scripts/third-party-licenses.sh", module)
		}
	}
}

func TestLicensesUnexpectedArgument(t *testing.T) {
	result := runCLI(t, "", "licenses", "--format", "json")
	if result.exitCode != 2 || result.stdout != "" || !strings.Contains(result.stderr, "licenses takes no arguments") {
		t.Errorf("got %+v, want exit 2, no stdout, stderr about the arguments", result)
	}
}

func TestLicensesInHelp(t *testing.T) {
	result := runCLI(t, "", "help")
	if !strings.Contains(result.stdout, "concourse-lint licenses") {
		t.Errorf("help does not mention licenses:\n%s", result.stdout)
	}
}
