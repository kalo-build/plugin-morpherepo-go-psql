package compile

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"

	"github.com/kalo-build/morphe-go/pkg/registry"
	"github.com/kalo-build/plugin-morpherepo-go-psql/pkg/compile/cfg"
	"github.com/kalo-build/plugin-morpherepo-go/pkg/repo"
)

// MorpheRepoToGoPSQL generates Go PostgreSQL repository implementations
// from .repo specification files and morphe model definitions.
func MorpheRepoToGoPSQL(config cfg.CompileConfig) error {
	// Load .repo specs
	specs, specsErr := repo.LoadRepoSpecs(config.RepoInputDirPath)
	if specsErr != nil {
		return fmt.Errorf("failed to load repo specs: %w", specsErr)
	}

	if len(specs) == 0 {
		return fmt.Errorf("no .repo files found in %s", config.RepoInputDirPath)
	}

	// Load morphe registry (for model field/relation information)
	r, rErr := registry.LoadMorpheRegistry(registry.LoadMorpheRegistryHooks{}, config.MorpheLoadRegistryConfig)
	if rErr != nil {
		return fmt.Errorf("failed to load morphe registry: %w", rErr)
	}

	allModels := r.GetAllModels()
	allEnums := r.GetAllEnums()

	// Ensure output directory exists
	if err := os.MkdirAll(config.OutputDirPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate a repository implementation for each spec
	for _, spec := range specs {
		model, ok := allModels[spec.Model]
		if !ok {
			return fmt.Errorf("morphe model %q not found in registry (referenced by %s)", spec.Model, spec.Name)
		}

		code := GenerateRepository(spec, model, allModels, allEnums, config)

		fileName := toSnakeCase(spec.Model) + "_repository.go"
		if err := writeFormattedGoFile(config.OutputDirPath, fileName, code); err != nil {
			return fmt.Errorf("failed to write repository for %s: %w", spec.Model, err)
		}
	}

	return nil
}

// writeFormattedGoFile formats Go source code and writes it to a file.
func writeFormattedGoFile(dirPath string, fileName string, code string) error {
	formatted, err := format.Source([]byte(code))
	if err != nil {
		// Write unformatted for debugging
		debugPath := filepath.Join(dirPath, fileName+".unformatted")
		_ = os.WriteFile(debugPath, []byte(code), 0644)
		return fmt.Errorf("format error in %s: %w", fileName, err)
	}

	filePath := filepath.Join(dirPath, fileName)
	return os.WriteFile(filePath, formatted, 0644)
}
