package lima

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

const (
	miB = 1 << 20
	// memoryCapBytes is the upper bound on VM memory; see vmMemory.
	memoryCapBytes = 8 << 30 // 8GiB
)

// vmMemory sizes the VM's memory as the lesser of memoryCapBytes and half the
// host's physical RAM. The build's footprint is stable, so the cap is plenty on
// a large host; halving keeps a small host responsive rather than overcommitting
// a fixed 8GiB it may not have.
func vmMemory() (string, error) {
	host, err := hostMemoryBytes()
	if err != nil {
		return "", fmt.Errorf("detecting host memory: %w", err)
	}
	return capMemory(host), nil
}

// capMemory applies the min(cap, host/2) rule and formats the result as a Lima
// memory string in MiB. Split out from vmMemory so the arithmetic is unit
// testable without touching the host.
func capMemory(hostBytes uint64) string {
	mem := uint64(memoryCapBytes)
	if half := hostBytes / 2; half < mem {
		mem = half
	}
	return fmt.Sprintf("%dMiB", mem/miB)
}

// hostMemoryBytes returns the host's total physical memory in bytes. It supports
// the two hosts this tool targets — Linux and macOS — and, like the rest of the
// package, shells out rather than linking C so the binary stays CGO-free.
func hostMemoryBytes() (uint64, error) {
	switch runtime.GOOS {
	case "linux":
		return linuxMemTotalBytes()
	case "darwin":
		return darwinMemSizeBytes()
	default:
		return 0, fmt.Errorf("host memory detection unsupported on %s", runtime.GOOS)
	}
}

// linuxMemTotalBytes reads MemTotal (reported in kB) from /proc/meminfo.
func linuxMemTotalBytes() (uint64, error) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	for line := range bytes.Lines(b) {
		rest, ok := bytes.CutPrefix(line, []byte("MemTotal:"))
		if !ok {
			continue
		}
		fields := bytes.Fields(rest) // e.g. ["16384000", "kB"]
		if len(fields) == 0 {
			return 0, fmt.Errorf("malformed MemTotal line: %q", line)
		}
		kb, err := strconv.ParseUint(string(fields[0]), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing MemTotal: %w", err)
		}
		return kb * 1024, nil
	}
	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}

// darwinMemSizeBytes reads hw.memsize (already in bytes) via sysctl.
func darwinMemSizeBytes() (uint64, error) {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, fmt.Errorf("sysctl hw.memsize: %w", err)
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing hw.memsize %q: %w", out, err)
	}
	return v, nil
}
