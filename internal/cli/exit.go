package cli

import "errors"

// Stable exit codes documented in AGENTS.md so AI agents and scripts
// can react to specific failure classes without parsing stderr.
//
// Update AGENTS.md if you add a new code so the contract stays honest.
const (
	// ExitOK is the default success exit code.
	ExitOK = 0

	// ExitValidation signals a user-fixable input problem: inventory
	// schema violation, missing required env var, bad flag value, etc.
	ExitValidation = 1

	// ExitNetwork signals an SSH / network failure where the remote host
	// couldn't be reached or the connection failed mid-command. Usually
	// transient — retrying is reasonable.
	ExitNetwork = 2

	// ExitApplyPartial signals that apply finished but at least one
	// host's role failed mid-execution. Other hosts may have completed
	// successfully — inspect the Apply output for details.
	ExitApplyPartial = 3
)

// exitCodeError carries a specific process exit code while signalling that
// any user-facing output has already been printed — Error() returns "" so
// main.go prints nothing extra (same convention as errSilent). Construct
// with errExit; read back with ExitCode.
type exitCodeError struct{ code int }

func (e exitCodeError) Error() string { return "" }

// errExit returns an error that makes the process exit with code, without
// printing a message (the command is expected to have already rendered its
// output, e.g. a results table or JSON). Lets a command request a
// documented code like ExitApplyPartial instead of the default 1.
func errExit(code int) error { return exitCodeError{code: code} }

// ExitCode maps err to a process exit code: an exitCodeError's own code,
// ExitOK for nil, otherwise 1. main.go calls this so the documented
// non-1 exit codes are actually emitted.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var ec exitCodeError
	if errors.As(err, &ec) {
		return ec.code
	}
	return 1
}
