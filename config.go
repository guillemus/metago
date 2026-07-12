package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type metagoConfig struct {
	templateArgs map[string]map[string]string
}

// loadMetagoConfig reads the single optional configuration file at the root passed to metago.
// Configuration only supplies named template argument defaults.
func loadMetagoConfig(root string) (metagoConfig, error) {
	path := filepath.Join(root, "metago.toml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return metagoConfig{templateArgs: map[string]map[string]string{}}, nil
	}
	if err != nil {
		return metagoConfig{}, fmt.Errorf("read metago.toml: %w", err)
	}

	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return metagoConfig{}, fmt.Errorf("metago.toml: %w", err)
	}
	config := metagoConfig{templateArgs: map[string]map[string]string{}}
	for key := range document {
		if key != "templates" {
			return metagoConfig{}, fmt.Errorf("metago.toml: unknown section %q; only template argument defaults are supported", key)
		}
	}
	templatesValue, ok := document["templates"]
	if !ok {
		return config, nil
	}
	templates, ok := templatesValue.(map[string]any)
	if !ok {
		return metagoConfig{}, fmt.Errorf("metago.toml: templates must be a table")
	}
	for templateName, templateValue := range templates {
		if templateName == "" {
			return metagoConfig{}, fmt.Errorf("metago.toml: template name must not be empty")
		}
		templateTable, ok := templateValue.(map[string]any)
		if !ok {
			return metagoConfig{}, fmt.Errorf("metago.toml: templates.%q must be a table", templateName)
		}
		for key := range templateTable {
			if key != "args" {
				return metagoConfig{}, fmt.Errorf("metago.toml: unknown key templates.%q.%s; only named argument defaults are supported", templateName, key)
			}
		}
		argsValue, ok := templateTable["args"]
		if !ok {
			continue
		}
		argsTable, ok := argsValue.(map[string]any)
		if !ok {
			return metagoConfig{}, fmt.Errorf("metago.toml: templates.%q.args must be a table", templateName)
		}
		args := make(map[string]string, len(argsTable))
		for key, value := range argsTable {
			stringValue, ok := value.(string)
			if !ok {
				return metagoConfig{}, fmt.Errorf("metago.toml: templates.%q.args.%s must be a string; template argument defaults currently support string values only", templateName, key)
			}
			if isReservedMetaArg(key) {
				return metagoConfig{}, fmt.Errorf("metago.toml: argument %q is reserved for future metago features", key)
			}
			args[key] = stringValue
		}
		config.templateArgs[templateName] = args
	}
	return config, nil
}
