package uping

import (
	"strings"
	"testing"
)

func TestNewUPingConfDefaults(t *testing.T) {
	conf := NewUPingConf()

	if conf.Size != 24 {
		t.Errorf("Size = %d, want 24", conf.Size)
	}

	if conf.Interval != 1 {
		t.Errorf("Interval = %d, want 1", conf.Interval)
	}

	if conf.Count != -1 || conf.CountSuccess != -1 {
		t.Errorf("Count/CountSuccess = (%d, %d), want (-1, -1)", conf.Count, conf.CountSuccess)
	}

	// TTL 0 means "let the OS choose", which is what the usage text promises.
	if conf.TTL != 0 {
		t.Errorf("TTL = %d, want 0 (OS default)", conf.TTL)
	}

	if errs := conf.Validate(); len(errs) != 0 {
		t.Errorf("defaults must validate, got %v", errs)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		adjust  func(*Conf)
		wantErr string
	}{
		{
			name:   "defaults",
			adjust: func(*Conf) {},
		},
		{
			name:    "source is not an IP",
			adjust:  func(c *Conf) { c.Source = "eth0" },
			wantErr: "Invalid source IP Address",
		},
		{
			name:   "source is an IP",
			adjust: func(c *Conf) { c.Source = "127.0.0.1" },
		},
		{
			name:   "source is an IPv6 IP",
			adjust: func(c *Conf) { c.Source = "::1" },
		},
		{
			name:    "two audible flags",
			adjust:  func(c *Conf) { c.AudibleSingle = true; c.AudibleAll = true },
			wantErr: "Invalid audible-flags combination",
		},
		{
			name:    "audible and inverse audible",
			adjust:  func(c *Conf) { c.AudibleSingle = true; c.ZudibleAll = true },
			wantErr: "Invalid audible-flags combination",
		},
		{
			name:   "single audible flag",
			adjust: func(c *Conf) { c.ZudibleSingle = true },
		},
		{
			name:    "interval zero",
			adjust:  func(c *Conf) { c.Interval = 0 },
			wantErr: "Invalid interval (1-300s)",
		},
		{
			name:    "interval too large",
			adjust:  func(c *Conf) { c.Interval = 301 },
			wantErr: "Invalid interval (1-300s)",
		},
		{
			name:    "interval negative",
			adjust:  func(c *Conf) { c.Interval = -1 },
			wantErr: "Invalid interval (1-300s)",
		},
		{
			name:   "interval bounds are inclusive",
			adjust: func(c *Conf) { c.Interval = 300 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := NewUPingConf()
			tt.adjust(&conf)

			errs := conf.Validate()

			if tt.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("unexpected errors: %v", errs)
				}
				return
			}

			if len(errs) != 1 || errs[0] != tt.wantErr {
				t.Fatalf("errors = %v, want exactly %q", errs, tt.wantErr)
			}
		})
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	conf := NewUPingConf()
	conf.Source = "not-an-ip"
	conf.Interval = 0
	conf.AudibleSingle = true
	conf.AudibleAll = true

	if errs := conf.Validate(); len(errs) != 3 {
		t.Errorf("got %d errors, want 3: %v", len(errs), errs)
	}
}

func TestNewUPingerRejectsInvalidConf(t *testing.T) {
	conf := NewUPingConf()
	conf.Interval = 0

	_, err := NewUPinger(conf)
	if err == nil {
		t.Fatal("expected an error for an invalid interval")
	}

	if !strings.Contains(err.Error(), "Invalid interval") {
		t.Errorf("error = %q, want it to mention the interval", err)
	}
}

func TestNewUPingerAcceptsValidConf(t *testing.T) {
	p, err := NewUPinger(NewUPingConf())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Fatal("got a nil pinger")
	}
}
