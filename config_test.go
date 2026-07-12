package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMetagoConfig(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "metago.toml"), `[templates."std.serde".args]
runtime = "example.com/project/internal/serdejson"
strict = "true"
`)

	config, err := loadMetagoConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	args := config.templateArgs["std.serde"]
	if args["runtime"] != "example.com/project/internal/serdejson" || args["strict"] != "true" {
		t.Fatalf("unexpected template defaults: %#v", args)
	}
}

func TestLoadMetagoConfigMissingIsEmpty(t *testing.T) {
	config, err := loadMetagoConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(config.templateArgs) != 0 {
		t.Fatalf("missing configuration produced defaults: %#v", config.templateArgs)
	}
}

func TestLoadMetagoConfigRequiresStringArguments(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "metago.toml"), `[templates."std.serde".args]
strict = true
`)

	_, err := loadMetagoConfig(dir)
	if err == nil || !strings.Contains(err.Error(), `templates."std.serde".args.strict must be a string`) ||
		!strings.Contains(err.Error(), "currently support string values only") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMetagoConfigIsReadOnlyFromProvidedRoot(t *testing.T) {
	parent := t.TempDir()
	writeTestFile(t, filepath.Join(parent, "metago.toml"), `[templates.example.args]
value = "parent"
`)
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}

	config, err := loadMetagoConfig(child)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.templateArgs) != 0 {
		t.Fatalf("configuration was inherited from parent: %#v", config.templateArgs)
	}
}

func TestConfiguredTemplateArgumentsAndExplicitOverride(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "metago.toml"), `[templates.example.args]
value = "configured"
other = "default"
`)
	writeTestFile(t, filepath.Join(dir, "templates.metago"), `{{ define "example" }}
const {{ name . }}Value = "{{ get .Args "value" }}:{{ arg "other" }}"
{{ end }}
`)
	writeTestFile(t, filepath.Join(dir, "model.go"), `package fixture

//mgo:gen example value=explicit
func First() {}

//mgo:gen example
func Second() {}
`)

	got, err := generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	source := string(got)
	for _, want := range []string{
		`const FirstValue = "explicit:default"`,
		`const SecondValue = "configured:default"`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("generated source missing %q:\n%s", want, source)
		}
	}
}
