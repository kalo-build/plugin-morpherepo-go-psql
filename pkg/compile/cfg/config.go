package cfg

import (
	"path"
	"strings"

	rcfg "github.com/kalo-build/morphe-go/pkg/registry/cfg"
)

// CompileConfig holds all configuration for the compilation process.
type CompileConfig struct {
	// MorpheLoadRegistryConfig for loading morphe models (fields, relations).
	MorpheLoadRegistryConfig rcfg.MorpheLoadRegistryConfig

	// RepoInputDirPath is the directory containing .repo YAML spec files.
	RepoInputDirPath string

	// OutputDirPath is where generated Go files are written.
	OutputDirPath string

	// ModelsConfig defines the Go models package path.
	ModelsConfig ModelsConfig

	// RepoConfig defines the Go repository package path.
	RepoConfig RepoConfig
}

// ModelsConfig holds the models package configuration.
type ModelsConfig struct {
	PackagePath string
}

// RepoConfig holds the repository package configuration.
type RepoConfig struct {
	PackagePath string
}

// PackageNameFromPath extracts the package name from a full Go import path.
func PackageNameFromPath(importPath string) string {
	if importPath == "" {
		return ""
	}
	parts := strings.Split(importPath, "/")
	return parts[len(parts)-1]
}

// RepoPackageName returns the package name for the repository.
func (c CompileConfig) RepoPackageName() string {
	return PackageNameFromPath(c.RepoConfig.PackagePath)
}

// ModelsPackageName returns the package name for the models.
func (c CompileConfig) ModelsPackageName() string {
	return PackageNameFromPath(c.ModelsConfig.PackagePath)
}

// ModelsImportPath returns the import path with proper quoting.
func (c CompileConfig) ModelsImportPath() string {
	return path.Clean(c.ModelsConfig.PackagePath)
}
