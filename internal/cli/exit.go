package cli

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
