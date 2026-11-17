package uping

import (
	"testing"
	"time"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		host    string
		user    string
		port    int
		wantErr bool
	}{
		{name: "host only", input: "example.com", host: "example.com"},
		{name: "address", input: "127.0.0.1", host: "127.0.0.1"},
		{name: "localhost", input: "localhost", host: "localhost"},
		{name: "user", input: "root@localhost", host: "localhost", user: "root"},
		{name: "user with underscore", input: "some_user@localhost", host: "localhost", user: "some_user"},
		{name: "user with dash", input: "some-user@localhost", host: "localhost", user: "some-user"},
		{name: "port", input: "localhost:2222", host: "localhost", port: 2222},
		{name: "user and port", input: "root@localhost:2222", host: "localhost", user: "root", port: 2222},
		{name: "address and port", input: "127.0.0.1:22", host: "127.0.0.1", port: 22},
		{name: "port one", input: "localhost:1", host: "localhost", port: 1},
		{name: "port max", input: "localhost:65535", host: "localhost", port: 65535},

		{name: "port zero", input: "localhost:0", wantErr: true},
		{name: "port too large", input: "localhost:65536", wantErr: true},
		{name: "missing host with port", input: ":22", wantErr: true},
		{name: "uppercase user", input: "Root@localhost", host: "localhost", user: "Root"},
		{name: "uppercase user and port", input: "Admin@localhost:22", host: "localhost", user: "Admin", port: 22},
		{name: "user starting with a digit", input: "1root@localhost", wantErr: true},
		{name: "unresolvable host", input: "this-host-does-not-exist.invalid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTarget(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseTarget(%q) = %+v, want an error", tt.input, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseTarget(%q) returned an unexpected error: %v", tt.input, err)
			}

			if got.Host != tt.host {
				t.Errorf("Host = %q, want %q", got.Host, tt.host)
			}

			if got.User != tt.user {
				t.Errorf("User = %q, want %q", got.User, tt.user)
			}

			if got.Port != tt.port {
				t.Errorf("Port = %d, want %d", got.Port, tt.port)
			}

			if got.Address == "" {
				t.Errorf("Address is empty for a resolvable host")
			}
		})
	}
}

// TestParseTargetLimitations documents the known parsing gaps so a future fix
// has a test to flip. These are limitations, not desired behaviour.
func TestParseTargetLimitations(t *testing.T) {
	t.Skip("documented limitation; remove this skip when IPv6 literals are supported")

	t.Run("IPv6 literal", func(t *testing.T) {
		if _, err := ParseTarget("::1"); err != nil {
			t.Skipf("IPv6 literals are not supported yet: %v", err)
		}
	})

	t.Run("IPv6 literal with port", func(t *testing.T) {
		if _, err := ParseTarget("[::1]:22"); err != nil {
			t.Skipf("bracketed IPv6 is not supported yet: %v", err)
		}
	})
}

func TestAddTargetWhileRunningFails(t *testing.T) {
	p, err := NewUPinger(NewUPingConf())
	if err != nil {
		t.Fatal(err)
	}

	if err := p.AddTarget("127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	// Start blocks, so drive the running state for real instead of poking the
	// (mutex-guarded) field directly.
	errc := make(chan error, 1)
	go func() { errc <- p.Start() }()

	deadline := time.Now().Add(2 * time.Second)
	for !p.IsRunning() {
		if time.Now().After(deadline) {
			p.Stop()
			t.Fatal("pinger did not start")
		}
		time.Sleep(time.Millisecond)
	}

	if err := p.AddTarget("localhost"); err == nil {
		t.Fatal("expected an error when adding a target to a running pinger")
	} else if err.Error() != "cannot add target while running" {
		t.Errorf("error = %q, want the running guard", err)
	}

	p.Stop()

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start() did not return after Stop()")
	}
}

func TestAddTargetRejectsInvalidTarget(t *testing.T) {
	p, err := NewUPinger(NewUPingConf())
	if err != nil {
		t.Fatal(err)
	}

	if err := p.AddTarget("this-host-does-not-exist.invalid"); err == nil {
		t.Fatal("expected an error for an unresolvable target")
	}

	if len(p.targets) != 0 {
		t.Errorf("len(targets) = %d, want 0", len(p.targets))
	}
}

func TestAddTargetAcceptsValidTarget(t *testing.T) {
	p, err := NewUPinger(NewUPingConf())
	if err != nil {
		t.Fatal(err)
	}

	if err := p.AddTarget("127.0.0.1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(p.targets) != 1 {
		t.Errorf("len(targets) = %d, want 1", len(p.targets))
	}
}
