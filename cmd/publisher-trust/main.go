package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/tako0614/takoform-forms/internal/publishertrust"
)

var errUsage = errors.New("usage: publisher-trust prepare --repository DIR --output DIR | verify-evidence --repository DIR --evidence DIR --expected-source-commit COMMIT | install --repository DIR --evidence DIR --expected-source-commit COMMIT | verify-set --repository DIR --set DIR | check --repository DIR")

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "publisher-trust:", err)
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) == 0 {
		return errUsage
	}
	switch arguments[0] {
	case "prepare":
		repository, target, err := parseTwoPaths("prepare", arguments[1:], "output")
		if err != nil {
			return err
		}
		report, err := publishertrust.PrepareSigningRequest(repository, target)
		if err != nil {
			return err
		}
		return writeJSON(output, report)
	case "verify-evidence":
		repository, evidence, commit, err := parseEvidence("verify-evidence", arguments[1:])
		if err != nil {
			return err
		}
		report, err := publishertrust.VerifySigningRequest(repository, evidence, commit)
		if err != nil {
			return err
		}
		return writeJSON(output, report)
	case "install":
		repository, evidence, commit, err := parseEvidence("install", arguments[1:])
		if err != nil {
			return err
		}
		if err := requireCleanExactHead(repository, commit); err != nil {
			return err
		}
		report, installedPath, err := publishertrust.InstallSigningRequest(repository, evidence, commit)
		if err != nil {
			return err
		}
		return writeJSON(output, struct {
			Status        string                            `json:"status"`
			InstalledPath string                            `json:"installedPath"`
			Verification  publishertrust.VerificationReport `json:"verification"`
		}{Status: "installed", InstalledPath: installedPath, Verification: report})
	case "verify-set":
		repository, setPath, err := parseTwoPaths("verify-set", arguments[1:], "set")
		if err != nil {
			return err
		}
		report, err := publishertrust.VerifyPublishedSet(repository, setPath)
		if err != nil {
			return err
		}
		return writeJSON(output, report)
	case "check":
		flags := flag.NewFlagSet("check", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		repository := ""
		flags.StringVar(&repository, "repository", "", "publisher repository root")
		if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 || repository == "" {
			return errUsage
		}
		reports, err := publishertrust.CheckPublishedSets(repository)
		if err != nil {
			return err
		}
		return writeJSON(output, struct {
			Status   string                              `json:"status"`
			SetCount int                                 `json:"setCount"`
			Sets     []publishertrust.VerificationReport `json:"sets"`
		}{Status: "verified", SetCount: len(reports), Sets: reports})
	default:
		return errUsage
	}
}

func parseTwoPaths(name string, arguments []string, secondName string) (string, string, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repository := ""
	second := ""
	flags.StringVar(&repository, "repository", "", "publisher repository root")
	flags.StringVar(&second, secondName, "", secondName+" path")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || repository == "" || second == "" {
		return "", "", errUsage
	}
	return repository, second, nil
}

func parseEvidence(name string, arguments []string) (string, string, string, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repository := ""
	evidence := ""
	commit := ""
	flags.StringVar(&repository, "repository", "", "publisher repository root")
	flags.StringVar(&evidence, "evidence", "", "external signed evidence directory")
	flags.StringVar(&commit, "expected-source-commit", "", "exact signed source commit")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || repository == "" || evidence == "" || commit == "" {
		return "", "", "", errUsage
	}
	return repository, evidence, commit, nil
}

func requireCleanExactHead(repository, expected string) error {
	head, err := gitOutput(repository, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != expected {
		return fmt.Errorf("repository HEAD %s differs from signed source commit %s", head, expected)
	}
	status, err := gitOutput(repository, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("repository must be clean before create-only trust-set installation:\n%s", status)
	}
	return nil
}

func gitOutput(repository string, arguments ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", repository}, arguments...)...)
	command.Stdin = nil
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w%s", strings.Join(arguments, " "), err, commandDetail(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func commandDetail(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return ": " + value
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
