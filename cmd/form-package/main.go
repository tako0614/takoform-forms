package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/tako0614/takoform/formpackage"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "form-package:", err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) == 0 {
		return usageError()
	}
	switch arguments[0] {
	case "verify":
		if len(arguments) != 2 {
			return usageError()
		}
		report, err := formpackage.VerifyDirectory(arguments[1])
		if err != nil {
			return err
		}
		verified, ok := report.VerifiedPackage()
		if !ok {
			return fmt.Errorf("verified Form Package capability was not issued")
		}
		locator, err := formpackage.PublicationLocatorFor(verified.PackageIndex(), verified.PackageDigest())
		if err != nil {
			return err
		}
		return writeJSON(output, locator)
	case "digest":
		if len(arguments) != 2 {
			return usageError()
		}
		raw, err := os.ReadFile(arguments[1])
		if err != nil {
			return err
		}
		digest, err := formpackage.DigestCanonicalJSON(raw)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, digest)
		return err
	default:
		return usageError()
	}
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func usageError() error {
	return fmt.Errorf("usage: form-package verify DIR | digest FILE")
}
