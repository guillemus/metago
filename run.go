package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type packageGeneration struct {
	dir   string
	files map[string][]byte
	stale []string
}

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

	// Build the complete desired state before changing the filesystem. A failure in any package
	// therefore leaves every package's previous generation intact.
	plans := make([]packageGeneration, 0, len(dirs))
	for _, dir := range dirs {
		logger.Debug("generating package", "dir", dir)
		files, err := generateFilesWithTemplates(dir, templateFiles, resolver)
		if err != nil {
			return err
		}
		stale, err := staleMetagoSidecars(dir, files)
		if err != nil {
			return err
		}
		plans = append(plans, packageGeneration{dir: dir, files: files, stale: stale})
	}

	for _, plan := range plans {
		if len(plan.files) == 0 && len(plan.stale) == 0 {
			logger.Debug("no meta comments or stale outputs found", "dir", plan.dir)
			continue
		}
		for _, output := range sortedMapKeys(plan.files) {
			src := plan.files[output]
			logger.Debug("writing generated file", "file", output, "bytes", len(src))
			if err := os.WriteFile(output, src, 0644); err != nil {
				return err
			}
			logger.Debug("generation complete", "file", output)
		}
		for _, stale := range plan.stale {
			logger.Debug("removing stale generated file", "file", stale)
			if err := os.Remove(stale); err != nil {
				return err
			}
		}
	}
	return nil
}

// staleMetagoSidecars returns generated sidecars owned by Metago that are absent from the desired
// output. Both the known filename shape and exact generated header are required before deletion.
func staleMetagoSidecars(dir string, desired map[string][]byte) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var stale []string
	for _, entry := range entries {
		if entry.IsDir() || !isGeneratedMetaFile(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if _, keep := desired[path]; keep {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if bytes.HasPrefix(src, []byte(generatedHeader)) {
			stale = append(stale, path)
		}
	}
	sort.Strings(stale)
	return stale, nil
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
