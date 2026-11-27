package report

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update rewrites the golden files instead of comparing against them:
//
//	go test ./internal/report -update
//
// Only use it after a deliberate format change, and review the diff: a golden
// file is the expected output, not a snapshot of whatever the code does.
var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

// compareGolden compares output with testdata/<name>.golden.
func compareGolden(t *testing.T, name string, output []byte) {
	t.Helper()
	goldenPath := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(goldenPath, output, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("%v (write it by hand, or run with -update after a deliberate change)", err)
	}
	if string(output) != string(expected) {
		t.Errorf("output differs from %s\ngot:\n%s\nwant:\n%s", goldenPath, output, expected)
	}
}
