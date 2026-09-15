package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, root, name, source string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerate(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module resetfixture\n\ngo 1.26.1\n")
	for _, name := range []string{"models.go", "models_test.go"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, root, name, string(data))
	}
	writeTestFile(t, root, "nested/model.go", "package nested\n// generate:reset\ntype Item struct { Value int }\n")
	writeTestFile(t, root, "plain/model.go", "package plain\ntype Item struct { Value int }\n")
	if err := generate(root); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(root, "reset.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(first, []byte(generatedHeader)) {
		t.Fatal("missing generated file header")
	}
	if _, err := os.Stat(filepath.Join(root, "nested/reset.gen.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "plain/reset.gen.go")); !os.IsNotExist(err) {
		t.Fatalf("unmarked package must not receive generated methods: %v", err)
	}
	if err := generate(root); err != nil {
		t.Fatalf("second generation: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(root, "reset.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("generation must be deterministic")
	}
	cmd := exec.Command("go", "test", "./...", "-count=1")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated methods failed: %v\n%s", err, out)
	}
}

func TestGenerateRejectsConflicts(t *testing.T) {
	for _, tc := range []struct{ name, source, existing, want string }{
		{"method", "// generate:reset\ntype Item struct{}\nfunc (*Item) Reset() {}", "", "already has a Reset"},
		{"file", "// generate:reset\ntype Item struct{}", "package fixture\n", "non-generated file"},
		{"nonstruct", "// generate:reset\ntype Item int", "", "requires a defined struct"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "go.mod", "module fixture\n\ngo 1.26.1\n")
			writeTestFile(t, root, "model.go", "package fixture\n"+tc.source+"\n")
			if tc.existing != "" {
				writeTestFile(t, root, "reset.gen.go", tc.existing)
			}
			if err := generate(root); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}
