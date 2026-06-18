package ups

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeServer runs a scripted NUT server on one end of a net.Pipe.
// script maps an exact command line (e.g. "LIST UPS") to a multi-line
// response (without the trailing newline on the final line). The
// server reads one command, writes its scripted response, repeats
// until the client closes the connection.
func fakeServer(t *testing.T, script map[string]string) (clientConn net.Conn) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { _ = a.Close() })

	go func() {
		defer func() { _ = b.Close() }()
		r := bufio.NewReader(b)
		for {
			line, err := r.ReadString('\n')
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
				return
			}
			if err != nil {
				t.Errorf("fake server read: %v", err)
				return
			}
			cmd := strings.TrimRight(line, "\r\n")
			resp, ok := script[cmd]
			if !ok {
				_, _ = io.WriteString(b, "ERR UNKNOWN-COMMAND\n")
				continue
			}
			if _, err := io.WriteString(b, resp+"\n"); err != nil {
				return
			}
		}
	}()
	return a
}

func newTestClient(t *testing.T, script map[string]string) *Client {
	t.Helper()
	conn := fakeServer(t, script)
	c := newClientFromConn(conn, 2*time.Second)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestListUPS(t *testing.T) {
	c := newTestClient(t, map[string]string{
		"LIST UPS": "BEGIN LIST UPS\nUPS myups \"Office UPS\"\nUPS rack \"Rack \\\"big\\\" UPS\"\nEND LIST UPS",
	})
	got, err := c.ListUPS()
	if err != nil {
		t.Fatalf("ListUPS: %v", err)
	}
	want := []UPS{
		{Name: "myups", Description: "Office UPS"},
		{Name: "rack", Description: `Rack "big" UPS`},
	}
	if len(got) != len(want) {
		t.Fatalf("ListUPS len = %d, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ListUPS[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestListVar(t *testing.T) {
	c := newTestClient(t, map[string]string{
		"LIST VAR myups": "BEGIN LIST VAR myups\nVAR myups battery.charge \"87\"\nVAR myups ups.status \"OL\"\nEND LIST VAR myups",
	})
	got, err := c.ListVar("myups")
	if err != nil {
		t.Fatalf("ListVar: %v", err)
	}
	if got["battery.charge"] != "87" {
		t.Errorf("battery.charge = %q, want 87", got["battery.charge"])
	}
	if got["ups.status"] != "OL" {
		t.Errorf("ups.status = %q, want OL", got["ups.status"])
	}
}

func TestGetVar(t *testing.T) {
	c := newTestClient(t, map[string]string{
		`GET VAR myups ups.status`: `VAR myups ups.status "OB LB"`,
	})
	v, err := c.GetVar("myups", "ups.status")
	if err != nil {
		t.Fatalf("GetVar: %v", err)
	}
	if v != "OB LB" {
		t.Errorf("GetVar = %q, want %q", v, "OB LB")
	}
}

func TestLogin(t *testing.T) {
	c := newTestClient(t, map[string]string{
		"USERNAME admin": "OK",
		"PASSWORD hunter2": "OK",
	})
	if err := c.Login("admin", "hunter2"); err != nil {
		t.Fatalf("Login: %v", err)
	}
}

func TestTypedErrors(t *testing.T) {
	cases := []struct {
		name   string
		cmd    string
		reply  string
		invoke func(*Client) error
		want   error
	}{
		{
			name:   "access denied on LOGIN",
			cmd:    "USERNAME bad",
			reply:  "ERR ACCESS-DENIED",
			invoke: func(c *Client) error { return c.Login("bad", "x") },
			want:   ErrAccessDenied,
		},
		{
			name:   "unknown UPS on LIST VAR",
			cmd:    "LIST VAR ghost",
			reply:  "ERR UNKNOWN-UPS",
			invoke: func(c *Client) error { _, err := c.ListVar("ghost"); return err },
			want:   ErrUnknownUPS,
		},
		{
			name:   "var not supported on GET VAR",
			cmd:    "GET VAR myups not.a.var",
			reply:  "ERR VAR-NOT-SUPPORTED",
			invoke: func(c *Client) error { _, err := c.GetVar("myups", "not.a.var"); return err },
			want:   ErrVarNotSupported,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, map[string]string{tc.cmd: tc.reply})
			err := tc.invoke(c)
			if !errors.Is(err, tc.want) {
				t.Errorf("got err = %v, want errors.Is %v", err, tc.want)
			}
		})
	}
}

func TestUnknownServerErrorWraps(t *testing.T) {
	c := newTestClient(t, map[string]string{
		"GET VAR myups battery.charge": "ERR DRIVER-NOT-CONNECTED extra-context",
	})
	_, err := c.GetVar("myups", "battery.charge")
	var ue *Error
	if !errors.As(err, &ue) {
		t.Fatalf("got err = %v, want *Error", err)
	}
	if ue.Reason != "DRIVER-NOT-CONNECTED" {
		t.Errorf("Reason = %q, want DRIVER-NOT-CONNECTED (trailing tokens stripped)", ue.Reason)
	}
}

func TestProtocolErrorOnMalformedFraming(t *testing.T) {
	c := newTestClient(t, map[string]string{
		"LIST UPS": "BEGIN LIST OF UPS\nEND LIST UPS",
	})
	_, err := c.ListUPS()
	if !errors.Is(err, ErrProtocol) {
		t.Errorf("got err = %v, want errors.Is(ErrProtocol)", err)
	}
}

func TestDialContextCancellation(t *testing.T) {
	// Use an unroutable TEST-NET-1 address with a tight timeout so the
	// dial actually fails fast on the test runner.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := Dial(ctx, "192.0.2.1:3493", DialOptions{Timeout: 50 * time.Millisecond})
	if err == nil {
		t.Fatal("Dial to unroutable address unexpectedly succeeded")
	}
}
