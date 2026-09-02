package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestHasUserDataDir(t *testing.T) {
	dir := "/home/me/.config/chatbang/profile_data"
	cmd := []byte("browseros\x00--user-data-dir=" + dir + "\x00--headless")
	if !hasUserDataDir(cmd, dir) {
		t.Fatal("expected match")
	}
	if hasUserDataDir(cmd, dir+"-extra") {
		t.Fatal("suffix should not match")
	}
	other := []byte("browseros\x00--user-data-dir=/home/me/.config/browser-os\x00")
	if hasUserDataDir(other, dir) {
		t.Fatal("other profile should not match")
	}
}

func TestRemoveSingletonFiles(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "SingletonLock")
	if err := os.Symlink("host-123", lock); err != nil {
		t.Fatal(err)
	}
	removeSingletonFiles(dir)
	if _, err := os.Lstat(lock); !os.IsNotExist(err) {
		t.Fatalf("lock still present: %v", err)
	}
}

func TestSingletonLockPID(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("karabala-Legion-5-15ITH6H-75053", filepath.Join(dir, "SingletonLock")); err != nil {
		t.Fatal(err)
	}
	pid, ok := singletonLockPID(dir)
	if !ok || pid != 75053 {
		t.Fatalf("got pid=%d ok=%v", pid, ok)
	}
}

func TestSnapScopeFromCgroup(t *testing.T) {
	raw := "0::/user.slice/user-1000.slice/user@1000.service/app.slice/snap.chromium.chromium-ee16c3e0-ea39-4c54-affe-afba09806df4.scope\n"
	got := snapScopeFromCgroup(raw)
	want := "snap.chromium.chromium-ee16c3e0-ea39-4c54-affe-afba09806df4.scope"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if snapScopeFromCgroup("0::/user.slice/user-1000.slice/session.slice") != "" {
		t.Fatal("expected empty for non-snap cgroup")
	}
}

func TestTerminateProfileProcesses(t *testing.T) {
	dir := t.TempDir()
	python, err := exec.LookPath("python3")
	if err != nil {
		python, err = exec.LookPath("python")
		if err != nil {
			t.Skip("python not available")
		}
	}
	cmd := exec.Command(python, "-c", "import time; time.sleep(30)", "--user-data-dir="+dir)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(pidsUsingProfile(dir)) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(pidsUsingProfile(dir)) == 0 {
		t.Fatal("expected to observe the sleep process")
	}
	if err := PrepareProfile(dir); err != nil {
		t.Fatal(err)
	}
	if leftover := pidsUsingProfile(dir); len(leftover) != 0 {
		t.Fatalf("leftover pids: %v", leftover)
	}
}
