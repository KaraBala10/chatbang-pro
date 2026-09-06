package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const maxChatInstances = 8

var profileSeedSkipDirs = map[string]bool{
	"Cache":              true,
	"Code Cache":         true,
	"GPUCache":           true,
	"DawnGraphiteCache":  true,
	"DawnWebGPUCache":    true,
	"ShaderCache":        true,
	"GrShaderCache":      true,
	"Service Worker":     true,
	"SingletonLock":      true,
	"SingletonSocket":    true,
	"SingletonCookie":    true,
}

// InstanceRecord tracks one running chatbang-pro process.
type InstanceRecord struct {
	Slot      int    `json:"slot"`
	PID       int    `json:"pid"`
	Profile   string `json:"profile"`
	StartedAt int64  `json:"startedAt"`
}

// ChatInstance is a registered running instance; call Release when the session ends.
type ChatInstance struct {
	stateDir string
	record   InstanceRecord
}

// Slot returns 0 for the main profile, 1+ for extra instance profiles.
func (c ChatInstance) Slot() int {
	if c.record.Slot < 0 {
		return 0
	}
	return c.record.Slot
}

// ProfileDir is the Chromium user-data directory for this instance.
func (c ChatInstance) ProfileDir() string {
	return c.record.Profile
}

// Release removes this instance from the registry.
func (c *ChatInstance) Release() {
	if c == nil || c.stateDir == "" || c.record.PID <= 0 {
		return
	}
	_ = withInstanceLock(c.stateDir, func() error {
		instances, err := readInstanceRegistry(c.stateDir)
		if err != nil {
			return err
		}
		next := instances[:0]
		for _, inst := range instances {
			if inst.PID != c.record.PID {
				next = append(next, inst)
			}
		}
		return writeInstanceRegistry(c.stateDir, next)
	})
	c.record.PID = 0
}

// RunningInstances returns live chatbang-pro instances after pruning dead PIDs.
func RunningInstances(stateDir string) ([]InstanceRecord, error) {
	var out []InstanceRecord
	err := withInstanceLock(stateDir, func() error {
		instances, err := readInstanceRegistry(stateDir)
		if err != nil {
			return err
		}
		out = pruneDeadInstances(instances)
		return writeInstanceRegistry(stateDir, out)
	})
	return out, err
}

// RunningInstanceCount reports how many chatbang-pro instances are currently running.
func RunningInstanceCount(stateDir string) (int, error) {
	instances, err := RunningInstances(stateDir)
	if err != nil {
		return 0, err
	}
	return len(instances), nil
}

// AcquireChatProfile picks the main profile or an extra slot when another instance is active.
func AcquireChatProfile(stateDir, baseProfile string) (*ChatInstance, error) {
	base, err := absProfile(baseProfile)
	if err != nil {
		return nil, err
	}
	var picked ChatInstance
	err = withInstanceLock(stateDir, func() error {
		instances, err := readInstanceRegistry(stateDir)
		if err != nil {
			return err
		}
		instances = pruneDeadInstances(instances)

		trySlot := func(slot int, profile string) error {
			if profileSlotReserved(instances, slot) {
				return fmt.Errorf("slot reserved")
			}
			if len(pidsUsingProfile(profile)) > 0 {
				return fmt.Errorf("profile busy")
			}
			if slot > 0 {
				if err := ensureInstanceProfile(base, profile); err != nil {
					return err
				}
			}
			if err := OpenProfile(profile); err != nil {
				return err
			}
			record := InstanceRecord{
				Slot:      slot,
				PID:       os.Getpid(),
				Profile:   profile,
				StartedAt: time.Now().Unix(),
			}
			instances = append(instances, record)
			if err := writeInstanceRegistry(stateDir, instances); err != nil {
				return err
			}
			picked = ChatInstance{stateDir: stateDir, record: record}
			return nil
		}

		if err := trySlot(0, base); err == nil {
			return nil
		}
		for slot := 1; slot <= maxChatInstances; slot++ {
			profile := instanceProfileDir(base, slot)
			if err := trySlot(slot, profile); err == nil {
				return nil
			}
		}
		return fmt.Errorf("up to %d chatbang-pro instances are already running; use --instances to check", maxChatInstances)
	})
	if err != nil {
		return nil, err
	}
	return &picked, nil
}

func instanceProfileDir(baseProfile string, slot int) string {
	root := filepath.Dir(baseProfile)
	return filepath.Join(root, "instances", strconv.Itoa(slot), "profile_data")
}

func instanceRegistryPath(stateDir string) string {
	return filepath.Join(stateDir, "instances.json")
}

func instanceLockPath(stateDir string) string {
	return filepath.Join(stateDir, "instances.lock")
}

func withInstanceLock(stateDir string, fn func() error) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(instanceLockPath(stateDir), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}

func readInstanceRegistry(stateDir string) ([]InstanceRecord, error) {
	path := instanceRegistryPath(stateDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var instances []InstanceRecord
	if len(data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(data, &instances); err != nil {
		return nil, err
	}
	return instances, nil
}

func writeInstanceRegistry(stateDir string, instances []InstanceRecord) error {
	if instances == nil {
		instances = []InstanceRecord{}
	}
	data, err := json.MarshalIndent(instances, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(instanceRegistryPath(stateDir), data, 0o644)
}

func profileSlotReserved(instances []InstanceRecord, slot int) bool {
	for _, inst := range instances {
		if inst.Slot == slot && instancePIDAlive(inst.PID) {
			return true
		}
	}
	return false
}

func pruneDeadInstances(instances []InstanceRecord) []InstanceRecord {
	if len(instances) == 0 {
		return nil
	}
	out := make([]InstanceRecord, 0, len(instances))
	for _, inst := range instances {
		if instancePIDAlive(inst.PID) {
			out = append(out, inst)
		}
	}
	return out
}

func instancePIDAlive(pid int) bool {
	if pid <= 1 || pid == os.Getpid() {
		return true
	}
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err != nil {
		return false
	}
	cmdline, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil || len(cmdline) == 0 {
		return false
	}
	parts := strings.Split(string(cmdline), "\x00")
	if len(parts) == 0 || parts[0] == "" {
		return false
	}
	base := filepath.Base(parts[0])
	return base == "chatbang-pro" || strings.HasPrefix(base, "chatbang-pro")
}

func ensureInstanceProfile(baseProfile, instProfile string) error {
	if err := os.MkdirAll(instProfile, 0o755); err != nil {
		return err
	}
	marker := filepath.Join(instProfile, ".seeded_from")
	if data, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(data)) == baseProfile {
		if fileExists(filepath.Join(instProfile, "Default", "Cookies")) {
			return nil
		}
	}
	srcDefault := filepath.Join(baseProfile, "Default")
	if st, err := os.Stat(srcDefault); err != nil || !st.IsDir() {
		return fmt.Errorf("main profile %s is not set up; run chatbang-pro --config first", baseProfile)
	}
	dstDefault := filepath.Join(instProfile, "Default")
	if err := copyProfileTree(srcDefault, dstDefault); err != nil {
		return err
	}
	_ = copyProfileFile(filepath.Join(baseProfile, "Local State"), filepath.Join(instProfile, "Local State"))
	_ = copyProfileFile(filepath.Join(baseProfile, "First Run"), filepath.Join(instProfile, "First Run"))
	return os.WriteFile(marker, []byte(baseProfile), 0o644)
}

func copyProfileTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyProfileFile(src, dst)
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, info.Mode())
		}
		parts := strings.Split(rel, string(os.PathSeparator))
		if len(parts) > 0 && profileSeedSkipDirs[parts[0]] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyProfileFile(path, target)
	})
}

func copyProfileFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = out.ReadFrom(in)
	return err
}
