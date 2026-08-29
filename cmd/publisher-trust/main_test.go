package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
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
	if report.Status != publishertrust.SigningRequiredStatus || report.PackageCount != 16 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestCommandsFailClosedOnIncompleteArguments(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{
		nil,
		{"prepare", "--repository", "."},
		{"verify-evidence", "--repository", ".", "--evidence", "candidate"},
		{"install", "--repository", ".", "--evidence", "candidate"},
		{"verify-set", "--repository", "."},
		{"check"},
		{"unknown"},
	} {
		if err := run(arguments, &bytes.Buffer{}); !errors.Is(err, errUsage) {
			t.Fatalf("arguments %v error = %v, want usage", arguments, err)
		}
	}
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
