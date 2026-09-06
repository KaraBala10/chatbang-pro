package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstanceProfileDirBesideMainProfile(t *testing.T) {
	base := "/home/karabala/chatbang/profile_data"
	got := instanceProfileDir(base, 2)
	want := "/home/karabala/chatbang/instances/2/profile_data"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRunningInstanceCountPrunesDeadPID(t *testing.T) {
	dir := t.TempDir()
	dead := InstanceRecord{Slot: 0, PID: 999999, Profile: filepath.Join(dir, "profile"), StartedAt: 1}
	if err := writeInstanceRegistry(dir, []InstanceRecord{dead}); err != nil {
		t.Fatal(err)
	}
	count, err := RunningInstanceCount(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
}

func TestEnsureInstanceProfileSeedsCookies(t *testing.T) {
	stateDir := t.TempDir()
	baseProfile := filepath.Join(stateDir, "main")
	instProfile := instanceProfileDir(baseProfile, 1)
	if err := os.MkdirAll(filepath.Join(baseProfile, "Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseProfile, "Default", "Cookies"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureInstanceProfile(baseProfile, instProfile); err != nil {
		t.Fatalf("ensureInstanceProfile: %v", err)
	}
	if err := OpenProfile(instProfile); err != nil {
		t.Fatalf("OpenProfile: %v", err)
	}
}

func TestAcquireChatProfileUsesMainThenExtraSlot(t *testing.T) {
	stateDir := t.TempDir()
	baseProfile := filepath.Join(stateDir, "main")
	if err := os.MkdirAll(filepath.Join(baseProfile, "Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseProfile, "Default", "Cookies"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := AcquireChatProfile(stateDir, baseProfile)
	if err != nil {
		t.Fatal(err)
	}
	if first.Slot() != 0 || first.ProfileDir() != baseProfile {
		t.Fatalf("first = slot %d profile %q", first.Slot(), first.ProfileDir())
	}

	second, err := AcquireChatProfile(stateDir, baseProfile)
	if err != nil {
		t.Fatal(err)
	}
	if second.Slot() != 1 {
		t.Fatalf("second slot = %d, want 1", second.Slot())
	}
	if second.ProfileDir() == baseProfile {
		t.Fatal("second instance should not reuse main profile")
	}
	if !fileExists(filepath.Join(second.ProfileDir(), "Default", "Cookies")) {
		t.Fatal("expected seeded cookies in extra profile")
	}

	count, err := RunningInstanceCount(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}

	first.Release()
	second.Release()
	count, err = RunningInstanceCount(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("count after release = %d, want 0", count)
	}
}

func TestInstancePIDAliveCurrentProcess(t *testing.T) {
	if !instancePIDAlive(os.Getpid()) {
		t.Fatal("current pid should be alive")
	}
}
