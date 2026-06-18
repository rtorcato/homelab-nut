package roles

import (
	"embed"
	"fmt"
	"io/fs"
)

// embeddedScripts bundles the bash scripts that the role
// implementations pipe over SSH. The wrap-then-port strategy:
// v1 ships the working bash; Phase 6 (#7) replaces with native Go.
//
// The build will fail if any of these paths doesn't exist — the role
// constructor reads them eagerly at init time.
//
//go:embed embedded/*.sh
var embeddedScripts embed.FS

// readScript returns the embedded script bytes for the given filename
// (just the basename, e.g. "setup-server.sh"). Used by role
// implementations to feed scripts to Connection.Pipe.
func readScript(name string) ([]byte, error) {
	data, err := fs.ReadFile(embeddedScripts, "embedded/"+name)
	if err != nil {
		return nil, fmt.Errorf("embedded script %q: %w", name, err)
	}
	return data, nil
}
