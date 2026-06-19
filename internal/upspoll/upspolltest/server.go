// Package upspolltest provides a fake NUT TCP server used by tests in
// internal/upspoll, internal/cli, and internal/tui. Exporting it once
// avoids three copies of the same protocol stub.
package upspolltest

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
)

// Server is a minimal fake upsd that answers LIST UPS and LIST VAR
// from the canned data passed to New. Unknown commands get
// "ERR UNKNOWN-COMMAND" so tests fail loudly when they misuse the API.
type Server struct {
	ln    net.Listener
	addr  string
	upses []string                     // body lines for LIST UPS, e.g. `UPS myups "Office"`
	vars  map[string]map[string]string // ups → var → value
}

// New listens on a random localhost port and returns a Server. The
// listener is closed via t.Cleanup so callers don't have to.
func New(t *testing.T, upses []string, vars map[string]map[string]string) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("upspolltest: listen: %v", err)
	}
	s := &Server{ln: ln, addr: ln.Addr().String(), upses: upses, vars: vars}
	go s.accept()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

// Addr returns the "127.0.0.1:PORT" the server is bound to. Pass this
// as inventory.Host.Address in tests.
func (s *Server) Addr() string { return s.addr }

func (s *Server) accept() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(c)
	}
}

func (s *Server) handle(c net.Conn) {
	defer func() { _ = c.Close() }()
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "LIST UPS":
			fmt.Fprint(c, "BEGIN LIST UPS\n")
			for _, u := range s.upses {
				fmt.Fprint(c, u+"\n")
			}
			fmt.Fprint(c, "END LIST UPS\n")
		case strings.HasPrefix(line, "LIST VAR "):
			ups := strings.TrimPrefix(line, "LIST VAR ")
			vs, ok := s.vars[ups]
			if !ok {
				fmt.Fprint(c, "ERR UNKNOWN-UPS\n")
				continue
			}
			fmt.Fprintf(c, "BEGIN LIST VAR %s\n", ups)
			for k, v := range vs {
				fmt.Fprintf(c, "VAR %s %s %q\n", ups, k, v)
			}
			fmt.Fprintf(c, "END LIST VAR %s\n", ups)
		default:
			fmt.Fprint(c, "ERR UNKNOWN-COMMAND\n")
		}
	}
}
