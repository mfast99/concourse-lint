// Package concourse checks the keys the linter knows against the structs of
// Concourse itself (package atc), so that a new Concourse release with new
// fields does not turn into CL018 false positives.
//
// It is a module of its own: atc pulls in a large dependency tree, which the
// main module and `go install` must not carry. The package has no code, only
// the test in conformance_test.go. Run it with `make conformance`.
//
// A new Concourse release: point the require of github.com/concourse/concourse
// in go.mod at the commit of the new tag (`go get
// github.com/concourse/concourse@<commit>`; the module has no /v8 suffix, so a
// tag cannot be used), then run `make conformance`.
package concourse
