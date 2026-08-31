package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tako0614/takoform-forms/internal/publishertrust"
)

const publicRepositoryURL = "https://github.com/tako0614/takoform-forms.git"

var errUsage = errors.New("usage: publisher-trust prepare --repository DIR --output DIR | prepare-advancement --repository DIR --previous-set COMMIT --statement-version SEMVER --output DIR | verify-evidence --repository DIR --evidence DIR --expected-source-commit COMMIT | install --repository DIR --evidence DIR --expected-source-commit COMMIT | recover-partial-install --repository DIR --set-id COMMIT | verify-set --repository DIR --set DIR | check --repository DIR")

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
	case "prepare-advancement":
		repository, previousSetID, statementVersion, target, err := parseAdvancement(arguments[1:])
		if err != nil {
			return err
		}
		report, err := preparePublicRevocationAdvancement(repository, previousSetID, statementVersion, target, publicRepositoryURL)
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
	case "recover-partial-install":
		repository, setID, err := parseTwoValues("recover-partial-install", arguments[1:], "set-id")
		if err != nil {
			return err
		}
		recovered, err := recoverPartialInstall(repository, setID, publicRepositoryURL)
		if err != nil {
			return err
		}
		return writeJSON(output, recovered)
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

func parseAdvancement(arguments []string) (string, string, string, string, error) {
	flags := flag.NewFlagSet("prepare-advancement", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repository := ""
	previousSet := ""
	statementVersion := ""
	output := ""
	flags.StringVar(&repository, "repository", "", "publisher repository root")
	flags.StringVar(&previousSet, "previous-set", "", "exact public predecessor set source commit")
	flags.StringVar(&statementVersion, "statement-version", "", "new revocation statement SemVer")
	flags.StringVar(&output, "output", "", "create-only external signing request directory")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || repository == "" || previousSet == "" || statementVersion == "" || output == "" {
		return "", "", "", "", errUsage
	}
	return repository, previousSet, statementVersion, output, nil
}

// preparePublicRevocationAdvancement derives the predecessor capability from a
// fresh credential-free checkout of the canonical public main and its
// immutable tags. A caller supplies only the predecessor set identity; no
// caller-provided checkpoint pin, verification report, or evidence directory
// is accepted.
func preparePublicRevocationAdvancement(repository, previousSetID, statementVersion, output, publicRepository string) (publishertrust.PreparationReport, error) {
	if !validCommitID(previousSetID) {
		return publishertrust.PreparationReport{}, fmt.Errorf("previous public set %q is not an exact lowercase nonzero commit", previousSetID)
	}
	repository, err := filepath.Abs(repository)
	if err != nil {
		return publishertrust.PreparationReport{}, fmt.Errorf("resolve repository root: %w", err)
	}
	status, err := gitOutput(repository, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return publishertrust.PreparationReport{}, err
	}
	if status != "" {
		return publishertrust.PreparationReport{}, fmt.Errorf("revocation signing preparation requires a clean public-main checkout:\n%s", status)
	}
	localHead, err := gitOutput(repository, "rev-parse", "HEAD")
	if err != nil || !validCommitID(localHead) {
		return publishertrust.PreparationReport{}, fmt.Errorf("revocation signing preparation cannot resolve an exact local HEAD")
	}
	remoteMainOutput, err := credentialFreeGitOutput(repository, "ls-remote", publicRepository, "refs/heads/main")
	if err != nil {
		return publishertrust.PreparationReport{}, fmt.Errorf("anonymous public main readback: %w", err)
	}
	publicMain, err := exactRemoteRef(remoteMainOutput, "refs/heads/main")
	if err != nil {
		return publishertrust.PreparationReport{}, fmt.Errorf("anonymous public main readback: %w", err)
	}
	if localHead != publicMain {
		return publishertrust.PreparationReport{}, fmt.Errorf("revocation signing checkout HEAD %s differs from public main %s", localHead, publicMain)
	}

	previousSetTag := "forms/sets/" + previousSetID
	previousSetTagOutput, err := credentialFreeGitOutput(repository,
		"ls-remote", "--tags", publicRepository,
		"refs/tags/"+previousSetTag, "refs/tags/"+previousSetTag+"^{}",
	)
	if err != nil {
		return publishertrust.PreparationReport{}, fmt.Errorf("anonymous predecessor set-tag readback: %w", err)
	}
	previousSetCommit, err := exactRemoteTagCommit(previousSetTagOutput, previousSetTag)
	if err != nil {
		return publishertrust.PreparationReport{}, fmt.Errorf("anonymous predecessor set-tag readback: %w", err)
	}

	temporary, err := os.MkdirTemp("", "takoform-revocation-predecessor-")
	if err != nil {
		return publishertrust.PreparationReport{}, fmt.Errorf("create public predecessor readback directory: %w", err)
	}
	defer os.RemoveAll(temporary)
	if _, err := credentialFreeGitOutput(repository,
		"clone", "--quiet", "--no-checkout", "--no-tags", "--depth=1", "--branch", "main", publicRepository, temporary,
	); err != nil {
		return publishertrust.PreparationReport{}, fmt.Errorf("fresh anonymous public predecessor clone: %w", err)
	}
	fetchedMain, err := gitOutput(temporary, "rev-parse", "HEAD")
	if err != nil || fetchedMain != publicMain {
		return publishertrust.PreparationReport{}, fmt.Errorf("fresh anonymous public main changed during predecessor readback")
	}
	if _, err := gitOutput(temporary, "checkout", "--quiet", "--detach", fetchedMain); err != nil {
		return publishertrust.PreparationReport{}, fmt.Errorf("materialize fresh anonymous public main: %w", err)
	}
	setRefspec := "refs/tags/" + previousSetTag + ":refs/tags/" + previousSetTag
	if _, err := credentialFreeGitOutput(temporary, "fetch", "--quiet", "--no-tags", publicRepository, setRefspec); err != nil {
		return publishertrust.PreparationReport{}, fmt.Errorf("fetch immutable predecessor set tag: %w", err)
	}
	fetchedSetCommit, err := gitOutput(temporary, "rev-parse", "refs/tags/"+previousSetTag+"^{commit}")
	if err != nil || fetchedSetCommit != previousSetCommit {
		return publishertrust.PreparationReport{}, fmt.Errorf("public predecessor set tag changed during anonymous readback")
	}
	setRelative := filepath.ToSlash(filepath.Join(publishertrust.TrustSetsRelativePath, previousSetID))
	if _, err := gitOutput(temporary, "diff", "--quiet", fetchedSetCommit, fetchedMain, "--", setRelative); err != nil {
		return publishertrust.PreparationReport{}, fmt.Errorf("public predecessor set bytes were updated or deleted after %s", previousSetTag)
	}
	setRoot := filepath.Join(temporary, filepath.FromSlash(setRelative))
	predecessor, err := publishertrust.VerifyPublishedSet(temporary, setRoot)
	if err != nil {
		return publishertrust.PreparationReport{}, fmt.Errorf("verify fresh anonymous predecessor set: %w", err)
	}
	if predecessor.SetID != previousSetID || predecessor.SetTag != previousSetTag {
		return publishertrust.PreparationReport{}, fmt.Errorf("fresh anonymous predecessor evidence does not equal %s", previousSetTag)
	}
	if err := verifyPublicRevocationPrefix(repository, temporary, publicRepository, fetchedMain, previousSetCommit, predecessor); err != nil {
		return publishertrust.PreparationReport{}, err
	}

	report, err := publishertrust.PrepareRevocationSigningRequest(temporary, setRoot, statementVersion, output)
	if err != nil {
		return publishertrust.PreparationReport{}, err
	}
	if err := requirePublicMainUnchanged(repository, publicRepository, publicMain); err != nil {
		return publishertrust.PreparationReport{}, err
	}
	return report, nil
}

func verifyPublicRevocationPrefix(repository, publicCheckout, publicRepository, publicMain, previousSetCommit string, predecessor publishertrust.VerificationReport) error {
	setInventory, err := credentialFreeGitOutput(repository, "ls-remote", "--tags", publicRepository, "refs/tags/forms/sets/*")
	if err != nil {
		return fmt.Errorf("anonymous public publisher-set inventory: %w", err)
	}
	publicSetNames, err := remoteTagNames(setInventory, "forms/sets/")
	if err != nil {
		return fmt.Errorf("anonymous public publisher-set inventory: %w", err)
	}
	expectedSetNames := make([]string, 0, len(predecessor.CheckpointHistory))
	for _, historical := range predecessor.CheckpointHistory {
		expectedSetNames = append(expectedSetNames, historical.SetTag)
	}
	sort.Strings(expectedSetNames)
	if strings.Join(publicSetNames, "\n") != strings.Join(expectedSetNames, "\n") {
		return fmt.Errorf("public publisher-set predecessor refs are %s, expected %s", printableList(publicSetNames), printableList(expectedSetNames))
	}
	setCommits := make(map[string]string, len(predecessor.CheckpointHistory))
	for index, historical := range predecessor.CheckpointHistory {
		output, err := credentialFreeGitOutput(repository,
			"ls-remote", "--tags", publicRepository,
			"refs/tags/"+historical.SetTag, "refs/tags/"+historical.SetTag+"^{}",
		)
		if err != nil {
			return fmt.Errorf("anonymous public publisher set %s readback: %w", historical.SetTag, err)
		}
		remoteCommit, err := exactRemoteTagCommit(output, historical.SetTag)
		if err != nil {
			return fmt.Errorf("anonymous public publisher set %s readback: %w", historical.SetTag, err)
		}
		setCommits[historical.SetTag] = remoteCommit
		refspec := "refs/tags/" + historical.SetTag + ":refs/tags/" + historical.SetTag
		if _, err := credentialFreeGitOutput(publicCheckout, "fetch", "--quiet", "--no-tags", publicRepository, refspec); err != nil {
			return fmt.Errorf("fetch immutable public publisher set %s: %w", historical.SetTag, err)
		}
		fetchedCommit, err := gitOutput(publicCheckout, "rev-parse", "refs/tags/"+historical.SetTag+"^{commit}")
		if err != nil || fetchedCommit != remoteCommit {
			return fmt.Errorf("public publisher set %s changed during anonymous readback", historical.SetTag)
		}
		if _, err := gitOutput(publicCheckout, "diff", "--quiet", fetchedCommit, publicMain, "--",
			filepath.ToSlash(filepath.Join(publishertrust.TrustSetsRelativePath, historical.SetID)),
		); err != nil {
			return fmt.Errorf("public publisher set bytes at %s were updated or deleted", historical.SetTag)
		}
		if index == len(predecessor.CheckpointHistory)-1 && fetchedCommit != previousSetCommit {
			return fmt.Errorf("public predecessor set %s changed between tag readbacks", historical.SetTag)
		}
	}

	remoteInventory, err := credentialFreeGitOutput(repository, "ls-remote", "--tags", publicRepository, "refs/tags/forms/revocations/v*")
	if err != nil {
		return fmt.Errorf("anonymous public revocation-tag inventory: %w", err)
	}
	remoteNames, err := remoteTagNames(remoteInventory, "forms/revocations/v")
	if err != nil {
		return fmt.Errorf("anonymous public revocation-tag inventory: %w", err)
	}
	expected := append([]string(nil), predecessor.RevocationTags...)
	sort.Strings(expected)
	if strings.Join(remoteNames, "\n") != strings.Join(expected, "\n") {
		return fmt.Errorf("public revocation predecessor refs are %s, expected %s", printableList(remoteNames), printableList(expected))
	}
	for index, tag := range predecessor.RevocationTags {
		output, err := credentialFreeGitOutput(repository,
			"ls-remote", "--tags", publicRepository,
			"refs/tags/"+tag, "refs/tags/"+tag+"^{}",
		)
		if err != nil {
			return fmt.Errorf("anonymous public revocation tag %s readback: %w", tag, err)
		}
		remoteCommit, err := exactRemoteTagCommit(output, tag)
		if err != nil {
			return fmt.Errorf("anonymous public revocation tag %s readback: %w", tag, err)
		}
		refspec := "refs/tags/" + tag + ":refs/tags/" + tag
		if _, err := credentialFreeGitOutput(publicCheckout, "fetch", "--quiet", "--no-tags", publicRepository, refspec); err != nil {
			return fmt.Errorf("fetch immutable public revocation tag %s: %w", tag, err)
		}
		fetchedCommit, err := gitOutput(publicCheckout, "rev-parse", "refs/tags/"+tag+"^{commit}")
		if err != nil || fetchedCommit != remoteCommit {
			return fmt.Errorf("public revocation tag %s changed during anonymous readback", tag)
		}
		version := predecessor.Statements[index].StatementVersion
		if _, err := gitOutput(publicCheckout, "diff", "--quiet", fetchedCommit, publicMain, "--",
			"forms/revocations/"+version+".json", "forms/revocations/checkpoints/"+version+".json",
		); err != nil {
			return fmt.Errorf("public revocation bytes at %s were updated or deleted", tag)
		}
		checkpointSetTag := predecessor.CheckpointHistory[index+1].SetTag
		if fetchedCommit != setCommits[checkpointSetTag] {
			return fmt.Errorf("public checkpoint set %s and revocation %s do not identify one atomic publication commit", checkpointSetTag, tag)
		}
	}
	return nil
}

func exactRemoteTagCommit(output, tag string) (string, error) {
	directRef := "refs/tags/" + tag
	peeledRef := directRef + "^{}"
	direct := ""
	peeled := ""
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !validCommitID(fields[0]) {
			return "", fmt.Errorf("tag %s returned invalid evidence", tag)
		}
		switch fields[1] {
		case directRef:
			if direct != "" {
				return "", fmt.Errorf("tag %s returned duplicate direct refs", tag)
			}
			direct = fields[0]
		case peeledRef:
			if peeled != "" {
				return "", fmt.Errorf("tag %s returned duplicate peeled refs", tag)
			}
			peeled = fields[0]
		default:
			return "", fmt.Errorf("tag %s returned unexpected ref %s", tag, fields[1])
		}
	}
	if direct == "" {
		return "", fmt.Errorf("tag %s is missing", tag)
	}
	if peeled != "" {
		return "", fmt.Errorf("tag %s is annotated; publisher identities must be lightweight commit refs", tag)
	}
	return direct, nil
}

func remoteTagNames(output, prefix string) ([]string, error) {
	direct := map[string]struct{}{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !validCommitID(fields[0]) || !strings.HasPrefix(fields[1], "refs/tags/"+prefix) {
			return nil, fmt.Errorf("public %s inventory returned invalid evidence", prefix)
		}
		name := strings.TrimPrefix(fields[1], "refs/tags/")
		if strings.HasSuffix(name, "^{}") {
			continue
		}
		if _, duplicate := direct[name]; duplicate {
			return nil, fmt.Errorf("public %s inventory repeats %s", prefix, name)
		}
		direct[name] = struct{}{}
		limit := publishertrust.MaxRevocationSequence
		if prefix == "forms/sets/" {
			limit++
		}
		if len(direct) > limit {
			return nil, fmt.Errorf("public %s inventory exceeds the bounded %d-ref publisher history", prefix, limit)
		}
	}
	names := make([]string, 0, len(direct))
	for name := range direct {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func printableList(values []string) string {
	if len(values) == 0 {
		return "<empty>"
	}
	return strings.Join(values, ", ")
}

func parseTwoValues(name string, arguments []string, secondName string) (string, string, error) {
	return parseTwoPaths(name, arguments, secondName)
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

type partialInstallRecoveryReport struct {
	Status        string `json:"status"`
	SetID         string `json:"setId"`
	RecoveredPath string `json:"recoveredPath"`
	PublicCommit  string `json:"publicCommit"`
}

func recoverPartialInstall(repository, setID, publicRepository string) (partialInstallRecoveryReport, error) {
	if !validCommitID(setID) {
		return partialInstallRecoveryReport{}, fmt.Errorf("partial install set-id %q is not an exact lowercase nonzero commit", setID)
	}
	repository, err := filepath.Abs(repository)
	if err != nil {
		return partialInstallRecoveryReport{}, fmt.Errorf("resolve repository root: %w", err)
	}
	relative := filepath.ToSlash(filepath.Join(publishertrust.TrustSetsRelativePath, setID))
	target := filepath.Join(repository, filepath.FromSlash(relative))
	snapshot, err := inspectPartialInstallTree(target)
	if err != nil {
		return partialInstallRecoveryReport{}, err
	}
	if err := proveLocalPartialInstallUncommitted(repository, relative, snapshot.fileCount); err != nil {
		return partialInstallRecoveryReport{}, err
	}
	if _, err := publishertrust.VerifyPublishedSet(repository, target); err == nil {
		return partialInstallRecoveryReport{}, fmt.Errorf("refusing to remove complete verified trust set %s", relative)
	}
	tag := "forms/sets/" + setID
	if err := requireLocalSetTagAbsent(repository, tag); err != nil {
		return partialInstallRecoveryReport{}, err
	}
	origin, err := gitOutput(repository, "remote", "get-url", "origin")
	if err != nil {
		return partialInstallRecoveryReport{}, err
	}
	if origin != publicRepository {
		return partialInstallRecoveryReport{}, fmt.Errorf("partial install recovery requires canonical public origin %s, found %s", publicRepository, origin)
	}
	if err := requirePublicSetTagAbsent(repository, publicRepository, tag); err != nil {
		return partialInstallRecoveryReport{}, err
	}
	temporary, err := os.MkdirTemp("", "takoform-partial-install-public-")
	if err != nil {
		return partialInstallRecoveryReport{}, fmt.Errorf("create public recovery readback directory: %w", err)
	}
	defer os.RemoveAll(temporary)
	if _, err := credentialFreeGitOutput(repository,
		"clone", "--quiet", "--branch", "main", "--tags", publicRepository, temporary,
	); err != nil {
		return partialInstallRecoveryReport{}, fmt.Errorf("fresh anonymous public main readback: %w", err)
	}
	publicCommit, err := gitOutput(temporary, "rev-parse", "HEAD")
	if err != nil || !validCommitID(publicCommit) {
		return partialInstallRecoveryReport{}, fmt.Errorf("fresh anonymous public main did not resolve an exact commit")
	}
	if _, err := os.Lstat(filepath.Join(temporary, filepath.FromSlash(relative))); err == nil {
		return partialInstallRecoveryReport{}, fmt.Errorf("refusing to remove %s because fresh public main already contains it", relative)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return partialInstallRecoveryReport{}, fmt.Errorf("inspect fresh public main trust set: %w", err)
	}
	publicHistory, err := gitOutput(temporary, "log", "--all", "--format=%H", "--", relative)
	if err != nil {
		return partialInstallRecoveryReport{}, fmt.Errorf("inspect fresh public history for trust set: %w", err)
	}
	if publicHistory != "" {
		return partialInstallRecoveryReport{}, fmt.Errorf("refusing to remove %s because reachable public history already contains it", relative)
	}
	// Repeat every mutable observation immediately before deletion. A public
	// set or local commit that appeared during readback makes recovery refuse.
	if err := requirePublicSetTagAbsent(repository, publicRepository, tag); err != nil {
		return partialInstallRecoveryReport{}, err
	}
	if err := requirePublicMainUnchanged(repository, publicRepository, publicCommit); err != nil {
		return partialInstallRecoveryReport{}, err
	}
	if err := requireLocalSetTagAbsent(repository, tag); err != nil {
		return partialInstallRecoveryReport{}, err
	}
	if err := proveLocalPartialInstallUncommitted(repository, relative, snapshot.fileCount); err != nil {
		return partialInstallRecoveryReport{}, err
	}
	currentSnapshot, err := inspectPartialInstallTree(target)
	if err != nil {
		return partialInstallRecoveryReport{}, err
	}
	if currentSnapshot != snapshot {
		return partialInstallRecoveryReport{}, fmt.Errorf("refusing partial install recovery because %s changed during readback", relative)
	}
	if err := os.RemoveAll(target); err != nil {
		return partialInstallRecoveryReport{}, fmt.Errorf("remove proven unpublished partial install: %w", err)
	}
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		return partialInstallRecoveryReport{}, fmt.Errorf("partial install recovery did not remove %s", relative)
	}
	return partialInstallRecoveryReport{
		Status: "unpublished-partial-removed", SetID: setID,
		RecoveredPath: target, PublicCommit: publicCommit,
	}, nil
}

func requirePublicMainUnchanged(repository, publicRepository, expected string) error {
	remote, err := credentialFreeGitOutput(repository, "ls-remote", publicRepository, "refs/heads/main")
	if err != nil {
		return fmt.Errorf("anonymous public main readback: %w", err)
	}
	commit, err := exactRemoteRef(remote, "refs/heads/main")
	if err != nil {
		return fmt.Errorf("anonymous public main readback: %w", err)
	}
	if commit != expected {
		return fmt.Errorf("refusing partial install recovery because public main changed during readback: %s -> %s", expected, commit)
	}
	return nil
}

func exactRemoteRef(output, expectedRef string) (string, error) {
	commit := ""
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != expectedRef || !validCommitID(fields[0]) || commit != "" {
			return "", fmt.Errorf("remote ref %s returned invalid evidence", expectedRef)
		}
		commit = fields[0]
	}
	if commit == "" {
		return "", fmt.Errorf("remote ref %s is missing", expectedRef)
	}
	return commit, nil
}

type partialInstallSnapshot struct {
	fileCount int
	digest    [sha256.Size]byte
}

func inspectPartialInstallTree(target string) (partialInstallSnapshot, error) {
	info, err := os.Lstat(target)
	if err != nil {
		return partialInstallSnapshot{}, fmt.Errorf("inspect partial install target: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return partialInstallSnapshot{}, fmt.Errorf("partial install target is not a regular directory")
	}
	files := 0
	hasher := sha256.New()
	err = filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == target {
			return nil
		}
		relative, err := filepath.Rel(target, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("partial install contains symlink %s", path)
		}
		if entry.IsDir() {
			_, _ = io.WriteString(hasher, "d\x00"+filepath.ToSlash(relative)+"\x00")
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("partial install contains unsupported entry %s", path)
		}
		files++
		_, _ = io.WriteString(hasher, "f\x00"+filepath.ToSlash(relative)+"\x00")
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(hasher, file); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		_, _ = io.WriteString(hasher, "\x00")
		return nil
	})
	if err != nil {
		return partialInstallSnapshot{}, err
	}
	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	return partialInstallSnapshot{fileCount: files, digest: digest}, nil
}

func proveLocalPartialInstallUncommitted(repository, relative string, fileCount int) error {
	tracked, err := gitOutput(repository, "ls-files", "--", relative)
	if err != nil {
		return err
	}
	if tracked != "" {
		return fmt.Errorf("refusing to remove tracked partial install %s", relative)
	}
	history, err := gitOutput(repository, "log", "--all", "--format=%H", "--", relative)
	if err != nil {
		return err
	}
	if history != "" {
		return fmt.Errorf("refusing to remove %s because a reachable local commit contains it", relative)
	}
	status, err := gitOutput(repository, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return err
	}
	lines := strings.Split(status, "\n")
	if status == "" {
		lines = nil
	}
	prefix := "?? " + relative + "/"
	for _, line := range lines {
		if !strings.HasPrefix(line, prefix) {
			return fmt.Errorf("partial install recovery requires an otherwise clean repository; found %s", line)
		}
	}
	if fileCount > 0 && len(lines) == 0 {
		return fmt.Errorf("partial install files are not visible as untracked Git state")
	}
	return nil
}

func requirePublicSetTagAbsent(repository, publicRepository, tag string) error {
	remote, err := credentialFreeGitOutput(repository, "ls-remote", "--tags", publicRepository, "refs/tags/"+tag, "refs/tags/"+tag+"^{}")
	if err != nil {
		return fmt.Errorf("anonymous public set-tag readback: %w", err)
	}
	if remote != "" {
		return fmt.Errorf("refusing to remove partial install because public set tag %s exists", tag)
	}
	return nil
}

func requireLocalSetTagAbsent(repository, tag string) error {
	command := exec.Command("git", "-C", repository, "show-ref", "--verify", "--quiet", "refs/tags/"+tag)
	command.Stdin = nil
	command.Env = credentialFreeGitEnvironment()
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err == nil {
		return fmt.Errorf("refusing to remove partial install because local set tag %s exists", tag)
	} else {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("cannot prove local set tag %s is absent: %w%s", tag, err, commandDetail(stderr.String()))
	}
}

func credentialFreeGitOutput(repository string, arguments ...string) (string, error) {
	gitArguments := []string{
		"-c", "credential.helper=", "-c", "http.extraHeader=", "-c", "protocol.file.allow=always",
	}
	gitArguments = append(gitArguments, arguments...)
	command := exec.Command("git", gitArguments...)
	command.Dir = repository
	command.Stdin = nil
	command.Env = credentialFreeGitEnvironment()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w%s", strings.Join(arguments, " "), err, commandDetail(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func credentialFreeGitEnvironment() []string {
	forbidden := []string{
		"GH_TOKEN=", "GITHUB_TOKEN=", "GIT_ASKPASS=", "GIT_CONFIG=", "GIT_CONFIG_COUNT=",
		"GIT_CONFIG_KEY_", "GIT_CONFIG_PARAMETERS=", "GIT_CONFIG_SYSTEM=", "GIT_CONFIG_VALUE_",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=", "GIT_COMMON_DIR=", "GIT_CONFIG_GLOBAL=",
		"GIT_CONFIG_NOSYSTEM=", "GIT_DIR=", "GIT_INDEX_FILE=", "GIT_NAMESPACE=",
		"GIT_OBJECT_DIRECTORY=", "GIT_REPLACE_REF_BASE=", "GIT_SHALLOW_FILE=",
		"GIT_OPTIONAL_LOCKS=", "GIT_PROXY_COMMAND=", "GIT_SSH=", "GIT_SSH_COMMAND=",
		"GIT_TERMINAL_PROMPT=", "GIT_WORK_TREE=", "HTTP_PROXY=", "HTTPS_PROXY=", "ALL_PROXY=",
		"NO_PROXY=", "http_proxy=", "https_proxy=", "all_proxy=", "no_proxy=", "NETRC=",
		"SSH_ASKPASS=", "SSH_AUTH_SOCK=",
	}
	environment := make([]string, 0, len(os.Environ())+4)
	for _, value := range os.Environ() {
		blocked := false
		for _, prefix := range forbidden {
			if strings.HasPrefix(value, prefix) {
				blocked = true
				break
			}
		}
		if !blocked {
			environment = append(environment, value)
		}
	}
	environment = append(environment,
		"GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0",
	)
	sort.Strings(environment)
	return environment
}

func validCommitID(value string) bool {
	if len(value) != 40 || value == strings.Repeat("0", 40) {
		return false
	}
	for _, character := range value {
		if character < '0' || (character > '9' && character < 'a') || character > 'f' {
			return false
		}
	}
	return true
}

func gitOutput(repository string, arguments ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", repository}, arguments...)...)
	command.Stdin = nil
	command.Env = credentialFreeGitEnvironment()
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
