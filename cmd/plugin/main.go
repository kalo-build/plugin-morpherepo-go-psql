package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	rcfg "github.com/kalo-build/morphe-go/pkg/registry/cfg"
	"github.com/kalo-build/plugin-morpherepo-go-psql/pkg/compile"
	"github.com/kalo-build/plugin-morpherepo-go-psql/pkg/compile/cfg"
)

type CompileConfigEntry struct {
	PackagePath string `json:"PackagePath"`
}

type StoreConfig struct {
	ID        uint32 `json:"id"`
	Type      string `json:"type"`
	MountPath string `json:"mountPath,omitempty"`
}

type PluginConfig struct {
	// Store-based paths (mounted by CLI for multi-input plugins)
	Stores map[string]StoreConfig `json:"stores,omitempty"`

	// Legacy single-input paths (for backward compatibility / direct invocation)
	InputPath  string `json:"inputPath,omitempty"`
	OutputPath string `json:"outputPath,omitempty"`

	Config  PluginConfigFields `json:"config"`
	Verbose bool               `json:"verbose,omitempty"`
}

type PluginConfigFields struct {
	Models             CompileConfigEntry `json:"models"`
	Repo               CompileConfigEntry `json:"repo"`
	MorpheRegistryPath string             `json:"morpheRegistryPath,omitempty"`
}

const (
	ErrMissingConfig       = 3
	ErrInvalidConfig       = 4
	ErrInputPathRequired   = 12
	ErrOutputPathRequired  = 13
	ErrPackagePathRequired = 14
	ErrRegistryRequired    = 15
	ErrCompileFailed       = 1
)

func logInfo(verbose bool, format string, args ...interface{}) {
	if verbose {
		fmt.Fprintf(os.Stdout, format+"\n", args...)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: plugin-morpherepo-go-psql <config>")
		fmt.Fprintln(os.Stderr, "  config: JSON string with store configurations")
		os.Exit(ErrMissingConfig)
	}

	rawConfig := os.Args[1]
	var pluginConfig PluginConfig
	if err := json.Unmarshal([]byte(rawConfig), &pluginConfig); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing config JSON:", err)
		os.Exit(ErrInvalidConfig)
	}

	// Determine paths - prefer store mounts, fall back to legacy paths
	var repoInputPath, registryPath, outputPath string

	if pluginConfig.Stores != nil {
		for _, store := range pluginConfig.Stores {
			switch store.MountPath {
			case "/repo":
				repoInputPath = "/repo"
			case "/registry":
				registryPath = "/registry"
			case "/input":
				// Single-input fallback
				if repoInputPath == "" {
					repoInputPath = "/input"
				}
			case "/output":
				outputPath = "/output"
			}
		}
	}

	// Fall back to legacy direct paths
	if repoInputPath == "" && pluginConfig.InputPath != "" {
		repoInputPath = pluginConfig.InputPath
	}
	if outputPath == "" && pluginConfig.OutputPath != "" {
		outputPath = pluginConfig.OutputPath
	}
	if registryPath == "" && pluginConfig.Config.MorpheRegistryPath != "" {
		registryPath = pluginConfig.Config.MorpheRegistryPath
	}

	// Validate required paths
	if repoInputPath == "" {
		fmt.Fprintln(os.Stderr, "Error: Input path is required (path to .repo files)")
		os.Exit(ErrInputPathRequired)
	}
	if outputPath == "" {
		fmt.Fprintln(os.Stderr, "Error: Output path is required")
		os.Exit(ErrOutputPathRequired)
	}
	if registryPath == "" {
		fmt.Fprintln(os.Stderr, "Error: Morphe registry path is required (mount /registry store or provide morpheRegistryPath config)")
		os.Exit(ErrRegistryRequired)
	}
	if pluginConfig.Config.Models.PackagePath == "" {
		fmt.Fprintln(os.Stderr, "Error: Models package path is required")
		os.Exit(ErrPackagePathRequired)
	}
	if pluginConfig.Config.Repo.PackagePath == "" {
		fmt.Fprintln(os.Stderr, "Error: Repo package path is required")
		os.Exit(ErrPackagePathRequired)
	}

	// Resolve absolute paths (only for non-mount paths)
	if repoInputPath[0] != '/' {
		if abs, err := filepath.Abs(repoInputPath); err == nil {
			repoInputPath = abs
		}
	}
	if outputPath[0] != '/' {
		if abs, err := filepath.Abs(outputPath); err == nil {
			outputPath = abs
		}
	}
	if registryPath[0] != '/' {
		if abs, err := filepath.Abs(registryPath); err == nil {
			registryPath = abs
		}
	}

	logInfo(pluginConfig.Verbose, "Loading .repo specs from: '%s'", repoInputPath)
	logInfo(pluginConfig.Verbose, "Loading morphe models from: '%s'", registryPath)
	logInfo(pluginConfig.Verbose, "Output Go PSQL repo to: '%s'", outputPath)

	compileConfig := cfg.CompileConfig{
		MorpheLoadRegistryConfig: rcfg.MorpheLoadRegistryConfig{
			RegistryEnumsDirPath:      filepath.Join(registryPath, "enums"),
			RegistryModelsDirPath:     filepath.Join(registryPath, "models"),
			RegistryStructuresDirPath: filepath.Join(registryPath, "structures"),
			RegistryEntitiesDirPath:   filepath.Join(registryPath, "entities"),
		},
		RepoInputDirPath: repoInputPath,
		OutputDirPath:    outputPath,
		ModelsConfig:     cfg.ModelsConfig{PackagePath: pluginConfig.Config.Models.PackagePath},
		RepoConfig:       cfg.RepoConfig{PackagePath: pluginConfig.Config.Repo.PackagePath},
	}

	logInfo(pluginConfig.Verbose, "Starting compilation process...")
	compileErr := compile.MorpheRepoToGoPSQL(compileConfig)
	if compileErr != nil {
		fmt.Fprintln(os.Stderr, "Compilation failed:", compileErr)
		os.Exit(ErrCompileFailed)
	}

	logInfo(pluginConfig.Verbose, "Compilation completed successfully")
	os.Exit(0)
}
