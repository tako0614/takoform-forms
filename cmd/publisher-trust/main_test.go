package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tako0614/takoform-forms/internal/publishertrust"
)

func TestPrepareCommandEmitsSigningRequiredReport(t *testing.T) {
	t.Parallel()
	output := filepath.Join(t.TempDir(), "request")
	var stdout bytes.Buffer
	if err := run([]string{
		"prepare",
		"--repository", filepath.Join("..", ".."),
		"--output", output,
	}, &stdout); err != nil {
		t.Fatal(err)
	}
	var report publishertrust.PreparationReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Status != publishertrust.SigningRequiredStatus || report.PackageCount != 17 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestCommandsFailClosedOnIncompleteArguments(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{
		nil,
		{"prepare", "--repository", "."},
		{"prepare-advancement", "--repository", ".", "--previous-set", "previous", "--statement-version", "1.0.0"},
		{"verify-evidence", "--repository", ".", "--evidence", "candidate"},
		{"install", "--repository", ".", "--evidence", "candidate"},
		{"recover-partial-install", "--repository", "."},
		{"verify-set", "--repository", "."},
		{"check"},
		{"unknown"},
	} {
		if err := run(arguments, &bytes.Buffer{}); !errors.Is(err, errUsage) {
			t.Fatalf("arguments %v error = %v, want usage", arguments, err)
		}
	}
}

func TestPrepareAdvancementRejectsCallerEvidencePaths(t *testing.T) {
	t.Parallel()
	output := filepath.Join(t.TempDir(), "request")
	_, err := preparePublicRevocationAdvancement(
		filepath.Join("..", ".."),
		filepath.Join(t.TempDir(), "caller-supplied-set"),
		"1.0.0",
		output,
		publicRepositoryURL,
	)
	if err == nil || !strings.Contains(err.Error(), "not an exact lowercase nonzero commit") {
		t.Fatalf("caller evidence path error = %v, want public set-id refusal", err)
	}
}

func TestCredentialFreeGitEnvironmentScrubsAuthorityAndRepositoryOverrides(t *testing.T) {
	for key, value := range map[string]string{
		"GITHUB_TOKEN":        "secret",
		"GIT_ASKPASS":         "/tmp/askpass",
		"GIT_CONFIG_GLOBAL":   "/tmp/config",
		"GIT_DIR":             "/tmp/other.git",
		"GIT_TERMINAL_PROMPT": "1",
		"GIT_WORK_TREE":       "/tmp/other-tree",
		"HTTPS_PROXY":         "https://user:secret@example.invalid",
		"SSH_AUTH_SOCK":       "/tmp/agent",
	} {
		t.Setenv(key, value)
	}
	environment := credentialFreeGitEnvironment()
	values := make(map[string]string, len(environment))
	for _, item := range environment {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[key] = value
		}
	}
	for _, key := range []string{"GITHUB_TOKEN", "GIT_ASKPASS", "GIT_DIR", "GIT_WORK_TREE", "HTTPS_PROXY", "SSH_AUTH_SOCK"} {
		if _, exists := values[key]; exists {
			t.Fatalf("credential-free environment retained %s", key)
		}
	}
	if values["GIT_TERMINAL_PROMPT"] != "0" || values["GIT_CONFIG_GLOBAL"] != "/dev/null" || values["GIT_CONFIG_NOSYSTEM"] != "1" {
		t.Fatalf("credential-free Git controls are incomplete: %+v", values)
	}
}

func TestRecoverPartialInstallRemovesOnlyUncommittedUnpublishedEvidence(t *testing.T) {
	t.Parallel()
	fixture := t.TempDir()
	public := filepath.Join(fixture, "public.git")
	source := filepath.Join(fixture, "source")
	working := filepath.Join(fixture, "working")
	gitTestRun(t, "", "init", "--bare", "--initial-branch=main", public)
	gitTestRun(t, "", "init", "--initial-branch=main", source)
	gitTestRun(t, source, "config", "user.name", "Publisher Trust Test")
	gitTestRun(t, source, "config", "user.email", "publisher-trust@example.invalid")
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, source, "add", "README.md")
	gitTestRun(t, source, "commit", "-m", "fixture")
	gitTestRun(t, source, "remote", "add", "origin", public)
	gitTestRun(t, source, "push", "-u", "origin", "main")
	gitTestRun(t, "", "clone", "--quiet", public, working)

	setID := "0123456789abcdef0123456789abcdef01234567"
	target := filepath.Join(working, "forms", "trust", "sets", setID)
	if err := os.MkdirAll(filepath.Join(target, "revocations"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "revocations", "checkpoint.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := recoverPartialInstall(working, setID, public)
	if err != nil {
		t.Fatalf("recover unpublished partial install: %v", err)
	}
	if report.Status != "unpublished-partial-removed" || report.SetID != setID || !validCommitID(report.PublicCommit) {
		t.Fatalf("unexpected recovery report: %+v", report)
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial target remains after recovery: %v", err)
	}

	localTagID := "1234567890abcdef1234567890abcdef12345678"
	localTagTarget := filepath.Join(working, "forms", "trust", "sets", localTagID)
	if err := os.MkdirAll(localTagTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localTagTarget, "partial"), []byte("local tag\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, working, "tag", "forms/sets/"+localTagID)
	if _, err := recoverPartialInstall(working, localTagID, public); err == nil || !strings.Contains(err.Error(), "local set tag") {
		t.Fatalf("local-tag recovery error = %v, want local-tag refusal", err)
	}
	if _, err := os.Lstat(localTagTarget); err != nil {
		t.Fatalf("local-tag partial target was removed: %v", err)
	}
	gitTestRun(t, working, "tag", "--delete", "forms/sets/"+localTagID)
	if err := os.RemoveAll(localTagTarget); err != nil {
		t.Fatal(err)
	}

	publicSetID := "89abcdef0123456789abcdef0123456789abcdef"
	publicTarget := filepath.Join(working, "forms", "trust", "sets", publicSetID)
	if err := os.MkdirAll(publicTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publicTarget, "partial"), []byte("partial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, source, "tag", "forms/sets/"+publicSetID)
	gitTestRun(t, source, "push", "origin", "refs/tags/forms/sets/"+publicSetID)
	if _, err := recoverPartialInstall(working, publicSetID, public); err == nil || !strings.Contains(err.Error(), "public set tag") {
		t.Fatalf("public-set recovery error = %v, want immutable-tag refusal", err)
	}
	if _, err := os.Lstat(publicTarget); err != nil {
		t.Fatalf("public-tagged partial target was removed: %v", err)
	}
	if err := os.RemoveAll(publicTarget); err != nil {
		t.Fatal(err)
	}

	publicMainSetID := "abcdef0123456789abcdef0123456789abcdef01"
	publicMainSource := filepath.Join(source, "forms", "trust", "sets", publicMainSetID)
	if err := os.MkdirAll(publicMainSource, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publicMainSource, "evidence"), []byte("public\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, source, "add", filepath.ToSlash(filepath.Join("forms", "trust", "sets", publicMainSetID)))
	gitTestRun(t, source, "commit", "-m", "publish set path without fixture tag")
	gitTestRun(t, source, "push", "origin", "main")
	localPublicMainTarget := filepath.Join(working, "forms", "trust", "sets", publicMainSetID)
	if err := os.MkdirAll(localPublicMainTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localPublicMainTarget, "partial"), []byte("partial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := recoverPartialInstall(working, publicMainSetID, public); err == nil || !strings.Contains(err.Error(), "public main already contains") {
		t.Fatalf("public-main recovery error = %v, want public-path refusal", err)
	}
	if _, err := os.Lstat(localPublicMainTarget); err != nil {
		t.Fatalf("public-main partial target was removed: %v", err)
	}
	if err := os.RemoveAll(localPublicMainTarget); err != nil {
		t.Fatal(err)
	}

	tagHistorySetID := "fedcba9876543210fedcba9876543210fedcba98"
	gitTestRun(t, source, "checkout", "--quiet", "-b", "tag-only-history")
	tagHistorySource := filepath.Join(source, "forms", "trust", "sets", tagHistorySetID)
	if err := os.MkdirAll(tagHistorySource, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tagHistorySource, "evidence"), []byte("tag history\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, source, "add", filepath.ToSlash(filepath.Join("forms", "trust", "sets", tagHistorySetID)))
	gitTestRun(t, source, "commit", "-m", "retain set path only through a public tag")
	gitTestRun(t, source, "tag", "archive/tag-only-set")
	gitTestRun(t, source, "push", "origin", "refs/tags/archive/tag-only-set")
	gitTestRun(t, source, "checkout", "--quiet", "main")
	gitTestRun(t, source, "branch", "-D", "tag-only-history")
	tagHistoryTarget := filepath.Join(working, "forms", "trust", "sets", tagHistorySetID)
	if err := os.MkdirAll(tagHistoryTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tagHistoryTarget, "partial"), []byte("partial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := recoverPartialInstall(working, tagHistorySetID, public); err == nil || !strings.Contains(err.Error(), "reachable public history") {
		t.Fatalf("tag-history recovery error = %v, want public-history refusal", err)
	}
	if _, err := os.Lstat(tagHistoryTarget); err != nil {
		t.Fatalf("tag-reachable partial target was removed: %v", err)
	}
}

func gitTestRun(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	if directory != "" {
		command.Dir = directory
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func TestBuiltCommandProvesReleasedCoreBeforeCheckingSets(t *testing.T) {
	command := exec.Command(
		"go", "run", ".", "check",
		"--repository", filepath.Join("..", ".."),
	)
	command.Env = append(command.Environ(), "GOWORK=off", "GOFLAGS=-mod=readonly")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run built publisher-trust command: %v\n%s", err, output)
	}
	var report struct {
		Status   string            `json:"status"`
		SetCount int               `json:"setCount"`
		Sets     []json.RawMessage `json:"sets"`
	}
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatal(err)
	}
	if report.Status != "verified" || report.SetCount != len(report.Sets) {
		t.Fatalf("unexpected built command report: %+v", report)
	}
}
