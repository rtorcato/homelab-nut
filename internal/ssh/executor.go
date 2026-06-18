package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/rtorcato/homelab-nut/internal/inventory"
	cryptossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Executor opens and caches SSH connections to inventory hosts.
//
// Open returns a *Connection — call Run or Stream on it to execute
// commands. Close shuts down every cached connection. The Executor is
// safe for concurrent use from multiple goroutines (each host gets a
// dedicated connection, multi-session over one TCP tunnel).
type Executor struct {
	cfg     Config
	mu      sync.Mutex
	conns   map[string]*Connection
	authMu  sync.Mutex
	authMs  []cryptossh.AuthMethod // resolved once, reused per connect
	authErr error
}

// NewExecutor returns an Executor with the given config. Pass
// NewConfig() if you want the recommended defaults.
func NewExecutor(cfg Config) *Executor {
	return &Executor{
		cfg:   cfg.applyDefaults(),
		conns: make(map[string]*Connection),
	}
}

// Open returns a Connection for host, opening one if needed. Repeated
// calls for the same host return the cached *Connection.
func (e *Executor) Open(h *inventory.Host) (*Connection, error) {
	if h == nil {
		return nil, errors.New("ssh: host is nil")
	}
	key := h.User + "@" + h.Address

	e.mu.Lock()
	if c, ok := e.conns[key]; ok {
		e.mu.Unlock()
		return c, nil
	}
	e.mu.Unlock()

	conn, err := e.dial(h)
	if err != nil {
		return nil, fmt.Errorf("ssh open %s: %w", key, err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	// Double-check under lock — another goroutine may have raced us.
	if existing, ok := e.conns[key]; ok {
		_ = conn.Close()
		return existing, nil
	}
	e.conns[key] = conn
	return conn, nil
}

// Close shuts down every cached connection. Safe to call concurrently
// with active connections; in-flight Run/Stream calls will error.
func (e *Executor) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var firstErr error
	for k, c := range e.conns {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(e.conns, k)
	}
	return firstErr
}

func (e *Executor) dial(h *inventory.Host) (*Connection, error) {
	auths, err := e.authMethods()
	if err != nil {
		return nil, err
	}

	hostKey, err := e.hostKeyCallback()
	if err != nil {
		return nil, err
	}

	addr := joinHostPort(h.Address, 22)

	clientCfg := &cryptossh.ClientConfig{
		User:            h.User,
		Auth:            auths,
		HostKeyCallback: hostKey,
		Timeout:         e.cfg.ConnectTimeout,
	}

	client, err := cryptossh.Dial("tcp", addr, clientCfg)
	if err != nil {
		return nil, err
	}
	return &Connection{
		client:     client,
		host:       h,
		cmdTimeout: e.cfg.CommandTimeout,
	}, nil
}

// authMethods resolves the AuthMethod list once and caches it. The
// agent socket and any key file are only opened the first time.
func (e *Executor) authMethods() ([]cryptossh.AuthMethod, error) {
	e.authMu.Lock()
	defer e.authMu.Unlock()
	if e.authMs != nil || e.authErr != nil {
		return e.authMs, e.authErr
	}

	var methods []cryptossh.AuthMethod

	if e.cfg.UseAgent {
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			c, err := net.Dial("unix", sock)
			if err == nil {
				methods = append(methods, cryptossh.PublicKeysCallback(agent.NewClient(c).Signers))
			}
			// If the socket exists but won't connect, fall through to key
			// file — better than failing the whole connect with an opaque
			// agent error.
		}
	}

	keyPath, keyErr := e.cfg.resolveKeyPath()
	if keyErr == nil {
		signer, err := loadPrivateKey(keyPath)
		if err == nil {
			methods = append(methods, cryptossh.PublicKeys(signer))
		}
	}

	if len(methods) == 0 {
		e.authErr = errors.New("ssh: no auth available — no usable ssh-agent and no readable key file (tried ~/.ssh/id_ed25519, ~/.ssh/id_rsa)")
		return nil, e.authErr
	}

	e.authMs = methods
	return methods, nil
}

// hostKeyCallback returns an SSH HostKeyCallback that consults
// known_hosts (strict mode) or accepts anything (relaxed). The result
// is *not* cached because the callback may need to re-read the file on
// each connect; the underlying knownhosts package does its own caching.
func (e *Executor) hostKeyCallback() (cryptossh.HostKeyCallback, error) {
	if !e.cfg.StrictHostKey {
		return cryptossh.InsecureIgnoreHostKey(), nil
	}
	path, err := e.cfg.resolveKnownHosts()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("ssh: known_hosts not found at %s — run `ssh-keyscan -H <host> >> %s` first, or pass StrictHostKey=false", path, path)
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("ssh: parse known_hosts %s: %w", path, err)
	}
	return cb, nil
}

func loadPrivateKey(path string) (cryptossh.Signer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key %s: %w", path, err)
	}
	signer, err := cryptossh.ParsePrivateKey(b)
	if err != nil {
		// Surface a clear error for passphrase-protected keys; we don't
		// prompt for passphrases (would break batch flows). Users should
		// add the key to ssh-agent instead.
		if _, ok := err.(*cryptossh.PassphraseMissingError); ok {
			return nil, fmt.Errorf("ssh: key %s is passphrase-protected — load it into ssh-agent with `ssh-add %s`", path, path)
		}
		return nil, fmt.Errorf("parse key %s: %w", path, err)
	}
	return signer, nil
}

// joinHostPort appends ":port" to host if not already present. Avoids
// pulling net.JoinHostPort's IPv6-bracketing edge cases since UPSes
// almost always live on IPv4 LANs; users can specify "[v6]:port"
// themselves if they need to.
func joinHostPort(addr string, defaultPort int) string {
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	return addr + ":" + strconv.Itoa(defaultPort)
}

// Connection wraps one underlying ssh.Client. Each call to Run/Stream
// opens a fresh session on top of the same TCP tunnel.
type Connection struct {
	client     *cryptossh.Client
	host       *inventory.Host
	cmdTimeout time.Duration
}

// Result is the combined output of a single command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Close terminates the underlying SSH client. Already-closed
// connections return nil.
func (c *Connection) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

// Host returns the inventory host this connection was opened for.
func (c *Connection) Host() *inventory.Host { return c.host }

// Run executes cmd, buffers stdout and stderr, and returns them
// together with the exit code. Non-zero exit is *not* a Go error —
// inspect Result.ExitCode for that. Network/protocol failures are.
func (c *Connection) Run(ctx context.Context, cmd string) (*Result, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("ssh: connection is closed")
	}
	sess, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("ssh: new session: %w", err)
	}
	defer sess.Close()

	ctx, cancel := c.applyCommandTimeout(ctx)
	defer cancel()
	out, errOut, exit, runErr := runSession(ctx, sess, cmd)
	if runErr != nil {
		return nil, runErr
	}
	return &Result{Stdout: out, Stderr: errOut, ExitCode: exit}, nil
}

// Pipe runs cmd with stdin from r, streaming stdout/stderr to the
// supplied writers. Convenience for the "embed a script, pipe it
// through ssh" pattern used by roles:
//
//	conn.Pipe(ctx, bytes.NewReader(scriptBytes),
//	    "sudo bash -s -- " + args, out, out)
//
// Like Stream, non-zero exits surface as *ssh.ExitError (use errors.As).
func (c *Connection) Pipe(ctx context.Context, stdin io.Reader, cmd string, stdout, stderr io.Writer) error {
	if c == nil || c.client == nil {
		return errors.New("ssh: connection is closed")
	}
	sess, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh: new session: %w", err)
	}
	defer sess.Close()

	if stdin != nil {
		sess.Stdin = stdin
	}
	if stdout != nil {
		sess.Stdout = stdout
	}
	if stderr != nil {
		sess.Stderr = stderr
	}

	ctx, cancel := c.applyCommandTimeout(ctx)
	defer cancel()
	return runStreamingSession(ctx, sess, cmd)
}

// Stream is like Run but writes stdout/stderr straight to the supplied
// writers as data arrives. Returns the same kind of "network failed"
// error as Run; non-zero exits are returned via *exec.ExitError-like
// wrapping (caller can inspect with errors.As → *ExitError).
func (c *Connection) Stream(ctx context.Context, cmd string, stdout, stderr io.Writer) error {
	if c == nil || c.client == nil {
		return errors.New("ssh: connection is closed")
	}
	sess, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh: new session: %w", err)
	}
	defer sess.Close()

	if stdout != nil {
		sess.Stdout = stdout
	}
	if stderr != nil {
		sess.Stderr = stderr
	}

	ctx, cancel := c.applyCommandTimeout(ctx)
	defer cancel()
	return runStreamingSession(ctx, sess, cmd)
}

// applyCommandTimeout returns ctx with the per-command timeout applied
// when configured. Always returns a non-nil cancel — the caller must
// defer it to avoid leaking the timer (no-op when cmdTimeout is 0).
func (c *Connection) applyCommandTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.cmdTimeout == 0 {
		return ctx, func() {}
	}
	// If the caller's ctx already has a tighter deadline, keep that.
	if d, ok := ctx.Deadline(); ok && time.Until(d) < c.cmdTimeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.cmdTimeout)
}

// runSession executes cmd on sess, buffering stdout/stderr. Returns
// exit=0 on success; exit=N (1..255) on non-zero exits; runErr is
// reserved for protocol/network failures.
func runSession(ctx context.Context, sess *cryptossh.Session, cmd string) (string, string, int, error) {
	stdoutBuf := &lineBuffer{}
	stderrBuf := &lineBuffer{}
	sess.Stdout = stdoutBuf
	sess.Stderr = stderrBuf

	err := runUnderContext(ctx, sess, cmd)
	switch e := err.(type) {
	case nil:
		return stdoutBuf.String(), stderrBuf.String(), 0, nil
	case *cryptossh.ExitError:
		return stdoutBuf.String(), stderrBuf.String(), e.ExitStatus(), nil
	default:
		return stdoutBuf.String(), stderrBuf.String(), -1, err
	}
}

// runStreamingSession runs cmd on a session that's already had its
// Stdout/Stderr wired up. Non-zero exits are returned as the ExitError.
func runStreamingSession(ctx context.Context, sess *cryptossh.Session, cmd string) error {
	return runUnderContext(ctx, sess, cmd)
}

// runUnderContext starts cmd, waits for it, and kills the session if
// ctx is cancelled mid-flight.
func runUnderContext(ctx context.Context, sess *cryptossh.Session, cmd string) error {
	done := make(chan error, 1)
	if err := sess.Start(cmd); err != nil {
		return fmt.Errorf("ssh: start: %w", err)
	}
	go func() { done <- sess.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = sess.Signal(cryptossh.SIGTERM)
		// Give the command a brief moment to clean up before we drop the
		// session entirely.
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		return ctx.Err()
	}
}

// lineBuffer is a tiny io.Writer that captures everything as a string.
// We use this rather than bytes.Buffer to keep imports minimal and to
// make it obvious the buffer is for short captured output (not large
// streams — use Stream() for those).
type lineBuffer struct{ buf []byte }

func (b *lineBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}
func (b *lineBuffer) String() string { return string(b.buf) }
