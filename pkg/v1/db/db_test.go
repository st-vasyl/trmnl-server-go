package db

import (
	"path/filepath"
	"testing"
)

// freshDB returns the path to a freshly-initialized SQLite DB in a temp dir.
func freshDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trmnl.db")
	if err := InitDB(path); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	return path
}

func TestInitDB_CreatesTable(t *testing.T) {
	path := freshDB(t)

	// Calling again on the same DB must be a no-op (CREATE TABLE IF NOT EXISTS).
	if err := InitDB(path); err != nil {
		t.Fatalf("second InitDB: %v", err)
	}

	// List of devices on an empty table should succeed.
	keys, err := GetDeviceList(path)
	if err != nil {
		t.Fatalf("GetDeviceList on empty DB: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("empty DB key count = %d, want 0", len(keys))
	}
}

func TestInitDB_InvalidPathReturnsError(t *testing.T) {
	// A path that points into a missing directory should fail to open.
	if err := InitDB("/this/path/does/not/exist/db.sqlite"); err == nil {
		t.Fatal("expected error for unreachable DB path, got nil")
	}
}

func TestRegisterDevice_RoundTrip(t *testing.T) {
	path := freshDB(t)

	if err := RegisterDevice(path, "dev-1", "key-1", "weather"); err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}

	screen, err := GetDeviceScreen(path, "dev-1")
	if err != nil {
		t.Fatalf("GetDeviceScreen: %v", err)
	}
	if screen != "weather" {
		t.Errorf("screen = %q, want %q", screen, "weather")
	}

	keys, err := GetDeviceList(path)
	if err != nil {
		t.Fatalf("GetDeviceList: %v", err)
	}
	if len(keys) != 1 || keys[0] != "key-1" {
		t.Errorf("keys = %v, want [key-1]", keys)
	}
}

func TestRegisterDevice_GeneratesApiKeyWhenEmpty(t *testing.T) {
	path := freshDB(t)

	if err := RegisterDevice(path, "dev-1", "", "weather"); err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}
	keys, err := GetDeviceList(path)
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
	path := freshDB(t)

	if _, err := GetDeviceScreen(path, "missing"); err == nil {
		t.Fatal("expected error for unknown device, got nil")
	}
}

func TestUpdateDevice_PersistsVoltageAndScreen(t *testing.T) {
	path := freshDB(t)
	if err := RegisterDevice(path, "dev-1", "key-1", "weather"); err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}

	if err := UpdateDevice(path, "dev-1", "4.1", "coingecko_bitcoin"); err != nil {
		t.Fatalf("UpdateDevice: %v", err)
	}

	screen, err := GetDeviceScreen(path, "dev-1")
	if err != nil {
		t.Fatalf("GetDeviceScreen: %v", err)
	}
	if screen != "coingecko_bitcoin" {
		t.Errorf("screen = %q, want %q", screen, "coingecko_bitcoin")
	}

	voltage, err := GetDeviceVoltage(path, "key-1")
	if err != nil {
		t.Fatalf("GetDeviceVoltage: %v", err)
	}
	if got := float32(4.1); !approx(voltage, got, 0.001) {
		t.Errorf("voltage = %v, want ~%v", voltage, got)
	}
}

func TestGetDeviceVoltage_UnknownKeyReturnsError(t *testing.T) {
	path := freshDB(t)
	if _, err := GetDeviceVoltage(path, "missing"); err == nil {
		t.Fatal("expected error for unknown api key, got nil")
	}
}

func TestGetDeviceList_MultipleDevices(t *testing.T) {
	path := freshDB(t)
	for _, k := range []string{"a", "b", "c"} {
		if err := RegisterDevice(path, "dev-"+k, "key-"+k, "weather"); err != nil {
			t.Fatalf("RegisterDevice %s: %v", k, err)
		}
	}
	keys, err := GetDeviceList(path)
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

func approx(a, b, eps float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}
