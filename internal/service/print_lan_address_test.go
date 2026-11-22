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

func TestNormalizeBluetoothMac(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"aa:bb:cc:dd:ee:ff", "AA:BB:CC:DD:EE:FF", true},
		{" 00:11:22:33:44:55 ", "00:11:22:33:44:55", true},
		{"AA-BB-CC-DD-EE-FF", "", false},
		{"AA:BB:CC:DD:EE", "", false},
		{"GG:BB:CC:DD:EE:FF", "", false},
		{"192.168.1.50", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeBluetoothMac(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("normalizeBluetoothMac(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestApplyPrinterBluetoothAndLan(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	t.Run("bluetooth receipt valid clears lan", func(t *testing.T) {
		lan := "192.168.1.50:9100"
		p := &Printer{Name: "Kasir BT", Station: "receipt", LanAddress: &lan, Copies: 1}
		err := applyPrinter(p, PrinterInput{
			TransportType: strPtr("bluetooth"),
			BluetoothMac:  strPtr("aa:bb:cc:dd:ee:ff"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.TransportType != "bluetooth" {
			t.Fatalf("transport = %q, want bluetooth", p.TransportType)
		}
		if p.BluetoothMac == nil || *p.BluetoothMac != "AA:BB:CC:DD:EE:FF" {
			t.Fatalf("mac = %v, want AA:BB:CC:DD:EE:FF", p.BluetoothMac)
		}
		if p.LanAddress != nil {
			t.Fatalf("lan = %v, want nil", p.LanAddress)
		}
	})

	t.Run("bluetooth requires mac", func(t *testing.T) {
		p := &Printer{Name: "Kasir BT", Station: "receipt", Copies: 1}
		err := applyPrinter(p, PrinterInput{TransportType: strPtr("bluetooth")})
		if err == nil {
			t.Fatal("expected validation error for missing bluetooth_mac")
		}
	})

	t.Run("bluetooth rejected on label station", func(t *testing.T) {
		p := &Printer{Name: "Label", Station: "label", Copies: 1}
		err := applyPrinter(p, PrinterInput{
			TransportType: strPtr("bluetooth"),
			BluetoothMac:  strPtr("AA:BB:CC:DD:EE:FF"),
		})
		if err == nil {
			t.Fatal("expected validation error for label over bluetooth")
		}
	})

	t.Run("switching back to lan clears bluetooth_mac", func(t *testing.T) {
		mac := "AA:BB:CC:DD:EE:FF"
		p := &Printer{Name: "Kasir", Station: "receipt", TransportType: "bluetooth", BluetoothMac: &mac, Copies: 1}
		err := applyPrinter(p, PrinterInput{
			TransportType: strPtr("lan"),
			LanAddress:    strPtr("192.168.1.88"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.BluetoothMac != nil {
			t.Fatalf("mac = %v, want nil", p.BluetoothMac)
		}
		if p.LanAddress == nil || *p.LanAddress != "192.168.1.88:9100" {
			t.Fatalf("lan = %v, want 192.168.1.88:9100", p.LanAddress)
		}
	})
}
