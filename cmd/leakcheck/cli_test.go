package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestCLI(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Exercise the real parser and the go vet handshake, not an argv helper.
	binary := filepath.Join(t.TempDir(), "leakcheck")
	if out, stderr, code := runCLI(t, "", "go", "build", "-o", binary, "."); code != 0 {
		t.Fatalf("build: exit %d\n%s%s", code, out, stderr)
	}
	fixture := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":        "module example.com/cliprobe\n\ngo 1.26.0\n",
		"probe_test.go": "package cliprobe\nimport \"testing\"\nfunc TestUncovered(t *testing.T) {}\n",
	} {
		if err := os.WriteFile(filepath.Join(fixture, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"ordinary", []string{"."}},
		{"version_as_file_value", []string{"-exclude-files", "-V", "."}},
		{"version_as_package_value", []string{"-exclude-packages", "--version", "."}},
		{"version_as_equals_value", []string{"-exclude-files=-V", "."}},
		{"version_disabled", []string{"-V=false", "."}},
		{"analyzer_flags", []string{"-concurrency=2", "-timeout=1m", "."}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, stderr, code := runCLI(t, fixture, binary, tc.args...)
			if code != 3 || !strings.Contains(stderr, "test function TestUncovered is not covered") || strings.Contains(out, "leakcheck has version") {
				t.Fatalf("args %q: exit %d\nstdout: %s\nstderr: %s", tc.args, code, out, stderr)
			}
		})
	}
	for _, args := range [][]string{{"--", "-V"}, {".", "-V"}, {"-unknown", "-V"}} {
		out, stderr, code := runCLI(t, fixture, binary, args...)
		if code == 0 || strings.Contains(out, "leakcheck has version") {
			t.Errorf("args %q: expected argument error, got exit %d\n%s%s", args, code, out, stderr)
		}
	}
	for _, arg := range []string{"-V", "--version"} {
		out, stderr, code := runCLI(t, fixture, binary, arg)
		if code != 0 || !strings.Contains(out, "leakcheck has version") {
			t.Errorf("%s: exit %d\n%s%s", arg, code, out, stderr)
		}
	}
	t.Run("vet_version", func(t *testing.T) {
		data, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		out, stderr, code := runCLI(t, fixture, binary, "-V=full")
		want := fmt.Sprintf(" version devel buildID=%x\n", sha256.Sum256(data))
		if code != 0 || !strings.HasSuffix(out, want) {
			t.Fatalf("-V=full: exit %d\n%s%s; want suffix %q", code, out, stderr, want)
		}
	})
	t.Run("flags_protocol", func(t *testing.T) {
		out, stderr, code := runCLI(t, fixture, binary, "-flags")
		var flags []struct {
			Name string
			Bool bool
		}
		if code != 0 || json.Unmarshal([]byte(out), &flags) != nil {
			t.Fatalf("-flags: exit %d\n%s%s", code, out, stderr)
		}
		got := make(map[string]bool)
		for _, flag := range flags {
			got[flag.Name] = flag.Bool
		}
		for name, isBool := range map[string]bool{"V": true, "version": true, "exclude-files": false, "exclude-packages": false, "json": true} {
			if value, ok := got[name]; !ok || value != isBool {
				t.Errorf("flag %s: present=%v, bool=%v; want bool=%v", name, ok, value, isBool)
			}
		}
	})
	t.Run("json", func(t *testing.T) {
		out, stderr, code := runCLI(t, fixture, binary, "-json", ".")
		if code != 0 || !json.Valid([]byte(out)) || !strings.Contains(out, "TestUncovered is not covered") {
			t.Fatalf("-json: exit %d\n%s%s", code, out, stderr)
		}
	})
	t.Run("fix_protocol", func(t *testing.T) {
		path := filepath.Join(fixture, "probe_test.go")
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out, stderr, code := runCLI(t, fixture, binary, "-fix", "-diff", ".")
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if code != 0 || !bytes.Equal(before, after) {
			t.Fatalf("-fix -diff: exit %d, source changed=%v\n%s%s", code, !bytes.Equal(before, after), out, stderr)
		}
	})
	t.Run("vet", func(t *testing.T) {
		out, stderr, code := runCLI(t, fixture, "go", "vet", "-vettool="+binary, ".")
		if code != 1 || !strings.Contains(stderr, "TestUncovered is not covered") {
			t.Fatalf("go vet: exit %d\n%s%s", code, out, stderr)
		}
	})
}

func runCLI(t *testing.T, dir, binary string, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("%s %q timed out: %s", binary, args, stderr.String())
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return stdout.String(), stderr.String(), exit.ExitCode()
		}
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), 0
}
