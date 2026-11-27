package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestWorkflowsUseGitHubHostedRunners guards the public repository: a pull
// request from a fork runs the workflow files of its own branch, so it could
// run any code on a self-hosted runner registered for the repository. Every
// job therefore runs on a GitHub-hosted runner.
func TestWorkflowsUseGitHubHostedRunners(t *testing.T) {
	workflowPaths, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(workflowPaths) == 0 {
		t.Fatal("no workflow files found")
	}
	for _, workflowPath := range workflowPaths {
		workflow, err := os.ReadFile(workflowPath)
		if err != nil {
			t.Fatal(err)
		}
		for lineIndex, line := range strings.Split(string(workflow), "\n") {
			trimmedLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimmedLine, "#") {
				continue
			}
			if strings.Contains(trimmedLine, "self-hosted") {
				t.Errorf("%s:%d: %q uses a self-hosted runner", filepath.Base(workflowPath), lineIndex+1, trimmedLine)
			}
		}
	}
}

// TestPullRequestWorkflowsCancelOutdatedRuns guards the concurrency of the
// workflows that run for pull requests: a new push to a pull request cancels
// the run of the commit before. Every other run (a push to main or develop, a
// schedule) gets a group of its own: in one shared group GitHub would cancel
// the waiting runs when more than one waits.
func TestPullRequestWorkflowsCancelOutdatedRuns(t *testing.T) {
	workflowPaths, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	wantLines := []string{
		"concurrency:",
		"group: ${{ github.workflow }}-${{ github.event_name == 'pull_request' && github.ref || github.run_id }}",
		"cancel-in-progress: true",
	}
	for _, workflowPath := range workflowPaths {
		workflow, err := os.ReadFile(workflowPath)
		if err != nil {
			t.Fatal(err)
		}
		var trimmedLines []string
		for _, line := range strings.Split(string(workflow), "\n") {
			trimmedLines = append(trimmedLines, strings.TrimSpace(line))
		}
		if !containsLine(trimmedLines, "pull_request:") {
			continue
		}
		for _, wantLine := range wantLines {
			if !containsLine(trimmedLines, wantLine) {
				t.Errorf("%s: runs for pull requests but has no %q", filepath.Base(workflowPath), wantLine)
			}
		}
	}
}

// TestReleaseJobsHaveTheirOwnNames guards the required status checks: a
// check is found by its name only, so a job of release.yml with the name of
// a job of another workflow (e.g. vscode) would turn that required check red
// on the commit of main whenever the release job fails, and block every pull
// request from main, such as merging main back into develop.
func TestReleaseJobsHaveTheirOwnNames(t *testing.T) {
	workflowPaths, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	releaseJobs := workflowJobNames(t, "../../.github/workflows/release.yml")
	if len(releaseJobs) == 0 {
		t.Fatal("no jobs found in release.yml")
	}
	for _, workflowPath := range workflowPaths {
		if filepath.Base(workflowPath) == "release.yml" {
			continue
		}
		for _, jobName := range workflowJobNames(t, workflowPath) {
			if slices.Contains(releaseJobs, jobName) {
				t.Errorf("release.yml and %s both have a job %q", filepath.Base(workflowPath), jobName)
			}
		}
	}
}

// workflowJobNames returns the IDs of the jobs of a workflow file: the keys
// indented by two spaces below "jobs:". A job with its own "name:" would be
// reported under that name, so none may have one.
func workflowJobNames(t *testing.T, workflowPath string) []string {
	t.Helper()
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	jobKey := regexp.MustCompile(`^  ([A-Za-z0-9_-]+):\s*$`)
	jobNameField := regexp.MustCompile(`^    name:`)
	var jobNames []string
	inJobs := false
	for lineIndex, line := range strings.Split(string(workflow), "\n") {
		if line == "jobs:" {
			inJobs = true
			continue
		}
		if !inJobs {
			continue
		}
		if match := jobKey.FindStringSubmatch(line); match != nil {
			jobNames = append(jobNames, match[1])
		}
		if jobNameField.MatchString(line) {
			t.Errorf("%s:%d: a job with its own name: the check would not carry the job ID", filepath.Base(workflowPath), lineIndex+1)
		}
	}
	return jobNames
}
