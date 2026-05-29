package db

import (
	"path/filepath"
	"testing"
)

// freshStore returns a freshly-opened Store backed by a temp SQLite file.
func freshStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trmnl.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpen_CreatesSchema(t *testing.T) {
	s := freshStore(t)

	// List on a freshly-opened (empty) DB must succeed.
	keys, err := s.GetDeviceList()
	if err != nil {
		t.Fatalf("GetDeviceList on empty DB: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("empty DB key count = %d, want 0", len(keys))
	}
}

func TestOpen_IsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trmnl.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	_ = s1.Close()

	// Second Open against the same file must not fail (CREATE TABLE IF NOT EXISTS).
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	_ = s2.Close()
}

func TestOpen_InvalidPathReturnsError(t *testing.T) {
	if _, err := Open("/this/path/does/not/exist/db.sqlite"); err == nil {
		t.Fatal("expected error for unreachable DB path, got nil")
	}
}

func TestRegisterDevice_RoundTrip(t *testing.T) {
	s := freshStore(t)

	if err := s.RegisterDevice("dev-1", "key-1", "weather"); err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}

	screen, err := s.GetDeviceScreen("dev-1")
	if err != nil {
		t.Fatalf("GetDeviceScreen: %v", err)
	}
	if screen != "weather" {
		t.Errorf("screen = %q, want %q", screen, "weather")
	}

	keys, err := s.GetDeviceList()
	if err != nil {
		t.Fatalf("GetDeviceList: %v", err)
	}
	if len(keys) != 1 || keys[0] != "key-1" {
		t.Errorf("keys = %v, want [key-1]", keys)
	}
}

func TestRegisterDevice_GeneratesApiKeyWhenEmpty(t *testing.T) {
	s := freshStore(t)

	if err := s.RegisterDevice("dev-1", "", "weather"); err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}
	keys, err := s.GetDeviceList()
	if err != nil {
		t.Fatalf("GetDeviceList: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if len(keys[0]) != 16 {
		t.Errorf("generated api key length = %d, want 16", len(keys[0]))
	}
}

func TestGetDeviceScreen_UnknownDeviceReturnsError(t *testing.T) {
	s := freshStore(t)

	if _, err := s.GetDeviceScreen("missing"); err == nil {
		t.Fatal("expected error for unknown device, got nil")
	}
}

func TestUpdateDevice_PersistsVoltageAndScreen(t *testing.T) {
	s := freshStore(t)
	if err := s.RegisterDevice("dev-1", "key-1", "weather"); err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}

	if err := s.UpdateDevice("dev-1", "4.1", "coingecko_bitcoin"); err != nil {
		t.Fatalf("UpdateDevice: %v", err)
	}

	screen, err := s.GetDeviceScreen("dev-1")
	if err != nil {
		t.Fatalf("GetDeviceScreen: %v", err)
	}
	if screen != "coingecko_bitcoin" {
		t.Errorf("screen = %q, want %q", screen, "coingecko_bitcoin")
	}

	voltage, err := s.GetDeviceVoltage("key-1")
	if err != nil {
		t.Fatalf("GetDeviceVoltage: %v", err)
	}
	if want := float32(4.1); !approx(voltage, want, 0.001) {
		t.Errorf("voltage = %v, want ~%v", voltage, want)
	}
}

func TestGetDeviceVoltage_UnknownKeyReturnsError(t *testing.T) {
	s := freshStore(t)
	if _, err := s.GetDeviceVoltage("missing"); err == nil {
		t.Fatal("expected error for unknown api key, got nil")
	}
}

func TestGetDeviceList_MultipleDevices(t *testing.T) {
	s := freshStore(t)
	for _, k := range []string{"a", "b", "c"} {
		if err := s.RegisterDevice("dev-"+k, "key-"+k, "weather"); err != nil {
			t.Fatalf("RegisterDevice %s: %v", k, err)
		}
	}
	keys, err := s.GetDeviceList()
	if err != nil {
		t.Fatalf("GetDeviceList: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("len(keys) = %d, want 3", len(keys))
	}
	set := map[string]bool{}
	for _, k := range keys {
		set[k] = true
	}
	for _, want := range []string{"key-a", "key-b", "key-c"} {
		if !set[want] {
			t.Errorf("missing key %q in %v", want, keys)
		}
	}
}

func TestClose_AllowsRepeatedSafely(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trmnl.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second Close on *sql.DB is documented as a no-op error or nil; either
	// is acceptable, but we should not panic.
	_ = s.Close()
}

func approx(a, b, eps float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}
