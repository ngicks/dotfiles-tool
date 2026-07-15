package lima

import (
	"runtime"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestInstanceYamlMarshalsValidConfig(t *testing.T) {
	c := Defaults()
	c.HostWork = "/h/work"
	b, err := c.instanceYaml()
	if err != nil {
		t.Fatalf("instanceYaml: %v", err)
	}
	var got instanceConfig
	if err := yaml.Unmarshal(b, &got); err != nil {
		t.Fatalf("output is not valid yaml: %v\n%s", err, b)
	}
	if got.Base != "template:docker" {
		t.Errorf("Base = %q", got.Base)
	}
	// vCPUs match the host; disk is the fixed size; memory is host-derived.
	if got.Cpus != runtime.NumCPU() {
		t.Errorf("Cpus = %d, want %d (host)", got.Cpus, runtime.NumCPU())
	}
	if got.Disk != "60GiB" {
		t.Errorf("Disk = %q", got.Disk)
	}
	if got.Memory == "" {
		t.Errorf("Memory is empty")
	}
	if len(got.Mounts) != 1 {
		t.Fatalf("Mounts = %+v", got.Mounts)
	}
	m := got.Mounts[0]
	if m.Location != "/h/work" || m.MountPoint != "/mnt/psbuild" || !m.Writable {
		t.Errorf("Mounts[0] = %+v", m)
	}
}

func TestInstanceYamlEmbedsDnsProvision(t *testing.T) {
	c := Defaults()
	c.HostWork = "/h/work"
	b, err := c.instanceYaml()
	if err != nil {
		t.Fatal(err)
	}
	var got instanceConfig
	if err := yaml.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Provision) != 1 || got.Provision[0].Mode != "system" {
		t.Fatalf("Provision = %+v", got.Provision)
	}
	script := got.Provision[0].Script
	for _, want := range []string{
		"DNS=1.1.1.1 8.8.8.8",
		"DNSStubListener=no",
		"systemctl restart systemd-resolved",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("provision script missing %q:\n%s", want, script)
		}
	}
}
