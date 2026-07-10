package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func run(root string) error {
	resolver, dirs, err := newResolver(root)
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		return fmt.Errorf("no Go package found in %s", root)
	}
	logger.Debug("found package directories", "count", len(dirs), "dirs", dirs)
	templateFiles, err := findTemplateFiles(root)
	if err != nil {
		return err
	}
	logger.Debug("found template files", "count", len(templateFiles), "files", templateFiles)

	for _, dir := range dirs {
		logger.Debug("generating package", "dir", dir)
		files, err := generateFilesWithTemplates(dir, templateFiles, resolver)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			logger.Debug("no meta comments found", "dir", dir)
			continue
		}

		outputs := sortedMapKeys(files)
		for _, output := range outputs {
			src := files[output]
			logger.Debug("writing generated file", "file", output, "bytes", len(src))
			if err := os.WriteFile(output, src, 0644); err != nil {
				return err
			}
			logger.Debug("generation complete", "file", output)
		}
	}
	return nil
}

func findPackageDirs(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && shouldSkipDir(entry.Name()) {
			return filepath.SkipDir
		}
		hasGoFiles, err := hasPackageGoFiles(path)
		if err != nil {
			return err
		}
		if hasGoFiles {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}

func shouldSkipDir(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".")
}

func findTemplateFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".metago") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func hasPackageGoFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || name == "meta.go" || strings.HasSuffix(name, "_meta.go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		return true, nil
	}
	return false, nil
}
