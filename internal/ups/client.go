// Package ups speaks the Network UPS Tools TCP protocol (port 3493) so
// `homelab-nut status` and the TUI Dashboard can read live UPS state
// without shelling out to `upsc`. The binary stays self-contained — the
// operator's laptop only needs `homelab-nut`, not nut-client.
//
// Scope: read-only commands needed for live status — LIST UPS,
// LIST VAR <ups>, GET VAR <ups> <var>, LOGIN. SET VAR (instcmd) and
// administrative commands like FSD are intentionally out of scope.
//
// Reference: https://networkupstools.org/docs/developer-guide.chunked/ar01s09.html
package ups

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// DefaultPort is the IANA-registered NUT upsd port.
const DefaultPort = 3493

// DialOptions configures Dial.
type DialOptions struct {
	// Timeout bounds the TCP connect. Zero means no client-side bound
	// (the OS connect timeout still applies).
	Timeout time.Duration

	// Deadline bounds every subsequent read and write on the connection.
	// Zero disables per-call deadlines, which is almost never what you
	// want for a network client — set it.
	Deadline time.Duration
}

// Client is a NUT protocol client. Use Dial to construct one. Not safe
// for concurrent use — open one Client per goroutine, or wrap calls in
// a mutex if you need to share.
type Client struct {
	conn     net.Conn
	r        *bufio.Reader
	deadline time.Duration
}

// Dial opens a TCP connection to a NUT upsd server. addr is host:port;
// if the port is omitted, DefaultPort is used. The returned Client
// owns the connection — call Close when done.
func Dial(ctx context.Context, addr string, opts DialOptions) (*Client, error) {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, fmt.Sprint(DefaultPort))
	}
	d := net.Dialer{Timeout: opts.Timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ups: dial %s: %w", addr, err)
	}
	return newClientFromConn(conn, opts.Deadline), nil
}

// newClientFromConn wraps a pre-established net.Conn — used by tests
// that drive the protocol through net.Pipe without touching the network.
func newClientFromConn(conn net.Conn, deadline time.Duration) *Client {
	return &Client{
		conn:     conn,
		r:        bufio.NewReader(conn),
		deadline: deadline,
	}
}

// Close closes the underlying TCP connection. Best-effort; the
// returned error is from net.Conn.Close and rarely useful — callers
// usually defer Close and ignore the result.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Login authenticates with USERNAME + PASSWORD. Most read-only
// installations don't require auth; only call this when the upsd
// config requires it or when a subsequent LOGIN command needs it.
func (c *Client) Login(user, password string) error {
	if _, err := c.command("USERNAME " + user); err != nil {
		return err
	}
	if _, err := c.command("PASSWORD " + password); err != nil {
		return err
	}
	return nil
}

// UPS describes one UPS exposed by upsd.
type UPS struct {
	Name        string
	Description string
}

// ListUPS returns every UPS the server knows about.
func (c *Client) ListUPS() ([]UPS, error) {
	lines, err := c.list("LIST UPS")
	if err != nil {
		return nil, err
	}
	out := make([]UPS, 0, len(lines))
	for _, line := range lines {
		toks, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		// Expected: UPS <name> "<description>"
		if len(toks) < 3 || toks[0] != "UPS" {
			return nil, fmt.Errorf("%w: unexpected UPS row %q", ErrProtocol, line)
		}
		out = append(out, UPS{Name: toks[1], Description: toks[2]})
	}
	return out, nil
}

// ListVar returns every variable exposed for the given UPS as a
// name → value map. Values are returned as raw strings — callers
// parse numerics themselves since NUT doesn't carry type info.
func (c *Client) ListVar(ups string) (map[string]string, error) {
	lines, err := c.list("LIST VAR " + ups)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(lines))
	for _, line := range lines {
		toks, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		// Expected: VAR <ups> <name> "<value>"
		if len(toks) < 4 || toks[0] != "VAR" || toks[1] != ups {
			return nil, fmt.Errorf("%w: unexpected VAR row %q", ErrProtocol, line)
		}
		out[toks[2]] = toks[3]
	}
	return out, nil
}

// GetVar reads a single variable. Use this when you only need one or
// two values — ListVar is cheaper if you want most of them.
func (c *Client) GetVar(ups, name string) (string, error) {
	line, err := c.command("GET VAR " + ups + " " + name)
	if err != nil {
		return "", err
	}
	toks, err := parseLine(line)
	if err != nil {
		return "", err
	}
	// Expected: VAR <ups> <name> "<value>"
	if len(toks) < 4 || toks[0] != "VAR" || toks[1] != ups || toks[2] != name {
		return "", fmt.Errorf("%w: unexpected GET VAR reply %q", ErrProtocol, line)
	}
	return toks[3], nil
}

// command writes a single line and returns the first response line.
// Returns a typed error if the server replies "ERR <reason>".
func (c *Client) command(cmd string) (string, error) {
	if err := c.write(cmd); err != nil {
		return "", err
	}
	line, err := c.readLine()
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(line, "ERR ") {
		reason := strings.TrimSpace(strings.TrimPrefix(line, "ERR "))
		// Some replies include a trailing detail token; the reason
		// itself is the first whitespace-delimited word.
		if i := strings.IndexByte(reason, ' '); i > 0 {
			reason = reason[:i]
		}
		return "", mapErr(reason)
	}
	return line, nil
}

// list runs a LIST-style command (LIST UPS, LIST VAR <ups>, …) and
// returns just the body lines — the BEGIN / END framing is stripped
// and validated.
func (c *Client) list(cmd string) ([]string, error) {
	first, err := c.command(cmd)
	if err != nil {
		return nil, err
	}
	wantBegin := "BEGIN " + cmd
	wantEnd := "END " + cmd
	if first != wantBegin {
		return nil, fmt.Errorf("%w: expected %q, got %q", ErrProtocol, wantBegin, first)
	}
	var body []string
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if line == wantEnd {
			return body, nil
		}
		body = append(body, line)
	}
}

func (c *Client) write(cmd string) error {
	if c.deadline > 0 {
		if err := c.conn.SetWriteDeadline(time.Now().Add(c.deadline)); err != nil {
			return fmt.Errorf("ups: set write deadline: %w", err)
		}
	}
	if _, err := io.WriteString(c.conn, cmd+"\n"); err != nil {
		return fmt.Errorf("ups: write %q: %w", cmd, err)
	}
	return nil
}

func (c *Client) readLine() (string, error) {
	if c.deadline > 0 {
		if err := c.conn.SetReadDeadline(time.Now().Add(c.deadline)); err != nil {
			return "", fmt.Errorf("ups: set read deadline: %w", err)
		}
	}
	line, err := c.r.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && line == "" {
			return "", io.EOF
		}
		return "", fmt.Errorf("ups: read: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
