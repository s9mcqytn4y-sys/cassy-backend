package service

import "testing"

func TestNormalizeLanAddress(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"192.168.1.50", "192.168.1.50:9100", true},
		{"10.0.0.7:9101", "10.0.0.7:9101", true},
		{"172.16.4.2:9100", "172.16.4.2:9100", true},
		{"8.8.8.8", "", false},
		{"127.0.0.1", "", false},
		{"192.168.1.50:0", "", false},
		{"192.168.1.50:70000", "", false},
		{"printer.local", "", false},
		{"[fd00::1]:9100", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeLanAddress(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("normalizeLanAddress(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}
