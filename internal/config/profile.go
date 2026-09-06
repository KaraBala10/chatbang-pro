package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var singletonFiles = []string{"SingletonLock", "SingletonSocket", "SingletonCookie"}

// OpenProfile prepares a profile directory when no browser is using it.
func OpenProfile(profileDir string) error {
	abs, err := absProfile(profileDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	if leftover := pidsUsingProfile(abs); len(leftover) > 0 {
		return fmt.Errorf("a browser is still using %s (pids %v)", abs, leftover)
	}
	removeSingletonFiles(abs)
	return nil
}

// PrepareProfile frees a Chromium user-data dir so a new instance can start.
// AppImage browsers often leave a child process behind after chromedp exits.
func PrepareProfile(profileDir string) error {
	abs, err := absProfile(profileDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	if err := terminateProfileProcesses(abs); err != nil {
		return err
	}
	if leftover := pidsUsingProfile(abs); len(leftover) > 0 {
		return fmt.Errorf("a browser is still using %s (pids %v); close it from a system terminal with:\n  pkill -f %q", abs, leftover, "--user-data-dir="+abs)
	}
	removeSingletonFiles(abs)
	return nil
}

// ReleaseProfile stops leftover browser processes for this profile and drops singleton files.
func ReleaseProfile(profileDir string) {
	abs, err := absProfile(profileDir)
	if err != nil {
		return
	}
	_ = terminateProfileProcesses(abs)
	if len(pidsUsingProfile(abs)) == 0 {
		removeSingletonFiles(abs)
	}
}

func absProfile(profileDir string) (string, error) {
	if profileDir == "" {
		return "", fmt.Errorf("profile directory is empty")
	}
	return filepath.Abs(profileDir)
}

func removeSingletonFiles(profileDir string) {
	for _, name := range singletonFiles {
		_ = os.Remove(filepath.Join(profileDir, name))
	}
}

func terminateProfileProcesses(profileDir string) error {
	pids := pidsUsingProfile(profileDir)
	if len(pids) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stderr, "Closing browser…")
	// Snap Chromium sandbox PIDs reject SIGTERM/SIGKILL (EPERM). Stop the
	// user unit first; that is what actually tears the leftover session down.
	stopSnapScopes(pids)
	if waitForProfileIdle(profileDir, 2*time.Second) {
		return nil
	}
	signalPIDs(pidsUsingProfile(profileDir), syscall.SIGTERM)
	if waitForProfileIdle(profileDir, 1500*time.Millisecond) {
		return nil
	}
	signalPIDs(pidsUsingProfile(profileDir), syscall.SIGKILL)
	_ = waitForProfileIdle(profileDir, 1500*time.Millisecond)
	return nil
}

func waitForProfileIdle(profileDir string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if len(pidsUsingProfile(profileDir)) == 0 {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func signalPIDs(pids []int, sig syscall.Signal) {
	self := os.Getpid()
	for _, pid := range pids {
		if pid <= 1 || pid == self {
			continue
		}
		_ = syscall.Kill(pid, sig)
	}
}

func pidsUsingProfile(profileDir string) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	self := os.Getpid()
	var pids []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil || len(cmdline) == 0 {
			continue
		}
		if hasUserDataDir(cmdline, profileDir) {
			pids = append(pids, pid)
		}
	}
	return pids
}

func hasUserDataDir(cmdline []byte, profileDir string) bool {
	needle := []byte("--user-data-dir=" + profileDir)
	i := bytes.Index(cmdline, needle)
	if i < 0 {
		return false
	}
	end := i + len(needle)
	if end >= len(cmdline) {
		return true
	}
	switch cmdline[end] {
	case 0, ' ', '\n':
		return true
	default:
		return false
	}
}

func singletonLockPID(profileDir string) (int, bool) {
	target, err := os.Readlink(filepath.Join(profileDir, "SingletonLock"))
	if err != nil {
		return 0, false
	}
	i := strings.LastIndex(target, "-")
	if i < 0 || i+1 >= len(target) {
		return 0, false
	}
	pid, err := strconv.Atoi(target[i+1:])
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func snapScopeFromCgroup(cgroup string) string {
	for _, line := range strings.Split(cgroup, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if i := strings.LastIndex(line, "/"); i >= 0 {
			line = line[i+1:]
		}
		if strings.HasPrefix(line, "snap.chromium.") && strings.HasSuffix(line, ".scope") {
			return line
		}
	}
	return ""
}

func stopSnapScopes(pids []int) {
	seen := map[string]bool{}
	for _, pid := range pids {
		if pid <= 1 {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cgroup"))
		if err != nil {
			continue
		}
		scope := snapScopeFromCgroup(string(raw))
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		cmd := exec.Command("systemctl", "--user", "stop", scope)
		cmd.Stdout = nil
		cmd.Stderr = nil
		_ = cmd.Run()
	}
}
