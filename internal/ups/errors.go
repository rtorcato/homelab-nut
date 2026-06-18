package ups

import "errors"

// Sentinel errors callers can match with errors.Is. Each maps to a
// distinct NUT protocol failure mode that callers usually want to
// branch on (e.g. retry vs. surface to user vs. reauth).
var (
	// ErrAccessDenied is returned when LOGIN/USERNAME/PASSWORD is
	// rejected by upsd or a command requires creds we didn't supply.
	// NUT protocol: "ERR ACCESS-DENIED".
	ErrAccessDenied = errors.New("ups: access denied")

	// ErrUnknownUPS is returned when a UPS name doesn't exist on the
	// server. NUT protocol: "ERR UNKNOWN-UPS".
	ErrUnknownUPS = errors.New("ups: unknown UPS")

	// ErrVarNotSupported is returned when a variable name isn't
	// implemented for the addressed UPS / driver.
	// NUT protocol: "ERR VAR-NOT-SUPPORTED".
	ErrVarNotSupported = errors.New("ups: variable not supported")

	// ErrProtocol is returned when the server replies with something
	// that doesn't fit the documented grammar (e.g. malformed BEGIN/END
	// framing, missing fields, unparseable quoted strings).
	ErrProtocol = errors.New("ups: protocol error")
)

// Error wraps a raw "ERR <reason>" response from upsd that didn't map
// to a known sentinel. Callers can inspect Reason for the raw token.
type Error struct {
	Reason string
}

func (e *Error) Error() string { return "ups: server error: " + e.Reason }
