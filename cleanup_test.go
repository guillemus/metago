package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRemovesSidecarWhenGenChangesToInline(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.go")
	writeTestFile(t, model, "package fixture\n\n//mgo:gen stringer\ntype Status string\n")
	writeTestFile(t, filepath.Join(dir, "templates.metago"), `{{ define "stringer" }}
func (s {{ name . }}) String() string { return string(s) }
{{ end }}
`)

	if err := run(dir); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(dir, "meta.go")
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("first run did not create sidecar: %v", err)
	}

	writeTestFile(t, model, "package fixture\n\n//mgo:inline stringer\ntype Status string\n")
	if err := run(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("stale sidecar still exists, stat error = %v", err)
	}
	src, err := os.ReadFile(model)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "func (s Status) String() string") || !strings.Contains(string(src), "//mgo:end") {
		t.Fatalf("inline output was not generated:\n%s", src)
	}
}

func TestRunRemovesSidecarsWhenDirectivesDisappear(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "model.go"), "package store\n\ntype Model struct{}\n")
	writeTestFile(t, filepath.Join(dir, "internal_test.go"), "package store\n\n//mgo:gen marker Internal\ntype Internal struct{}\n")
	writeTestFile(t, filepath.Join(dir, "external_test.go"), "package store_test\n\n//mgo:gen marker External\ntype External struct{}\n")
	writeTestFile(t, filepath.Join(dir, "templates.metago"), `{{ define "marker" }}const Generated{{ name . }} = true{{ end }}`)

	if err := run(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"meta_test.go", "meta_store_test.go"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("first run did not create %s: %v", name, err)
		}
	}

	writeTestFile(t, filepath.Join(dir, "internal_test.go"), "package store\n\ntype Internal struct{}\n")
	writeTestFile(t, filepath.Join(dir, "external_test.go"), "package store_test\n\ntype External struct{}\n")
	if err := run(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"meta_test.go", "meta_store_test.go"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("stale %s still exists, stat error = %v", name, err)
		}
	}
}

func TestRunPreservesUnownedGeneratedFilename(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "model.go"), "package fixture\n\ntype Model struct{}\n")
	path := filepath.Join(dir, "meta.go")
	const source = "package fixture\n\nconst HandWritten = true\n"
	writeTestFile(t, path, source)

	if err := run(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != source {
		t.Fatalf("unowned meta.go changed:\n%s", got)
	}
}

func TestRunGenerationFailureDoesNotRemoveEarlierPackageOutputs(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "a")
	second := filepath.Join(root, "b")
	writeTestFile(t, filepath.Join(first, "model.go"), "package a\n\ntype A struct{}\n")
	stale := filepath.Join(first, "meta.go")
	writeTestFile(t, stale, generatedHeader+"\npackage a\n\nconst Old = true\n")
	writeTestFile(t, filepath.Join(second, "model.go"), "package b\n\n//mgo:gen missing\ntype B struct{}\n")

	if err := run(root); err == nil {
		t.Fatal("expected generation failure")
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("generation failure removed an earlier package output: %v", err)
	}
}
