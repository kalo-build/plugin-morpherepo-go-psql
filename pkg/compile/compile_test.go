package compile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kalo-build/go-util/assertfile"
	rcfg "github.com/kalo-build/morphe-go/pkg/registry/cfg"
	"github.com/kalo-build/plugin-morpherepo-go-psql/internal/testutils"
	"github.com/kalo-build/plugin-morpherepo-go-psql/pkg/compile"
	"github.com/kalo-build/plugin-morpherepo-go-psql/pkg/compile/cfg"
)

type CompileTestSuite struct {
	assertfile.FileSuite

	TestDirPath            string
	TestGroundTruthDirPath string

	ModelsDirPath     string
	EnumsDirPath      string
	StructuresDirPath string
	EntitiesDirPath   string
	RepoDirPath       string
}

func TestCompileTestSuite(t *testing.T) {
	suite.Run(t, new(CompileTestSuite))
}

func (suite *CompileTestSuite) SetupTest() {
	suite.TestDirPath = testutils.GetTestDirPath()
	suite.TestGroundTruthDirPath = filepath.Join(suite.TestDirPath, "ground-truth", "compile-minimal")

	suite.ModelsDirPath = filepath.Join(suite.TestDirPath, "registry", "minimal", "models")
	suite.EnumsDirPath = filepath.Join(suite.TestDirPath, "registry", "minimal", "enums")
	suite.StructuresDirPath = filepath.Join(suite.TestDirPath, "registry", "minimal", "structures")
	suite.EntitiesDirPath = filepath.Join(suite.TestDirPath, "registry", "minimal", "entities")
	suite.RepoDirPath = filepath.Join(suite.TestDirPath, "repo", "minimal")
}

func (suite *CompileTestSuite) TearDownTest() {
	suite.TestDirPath = ""
}

func (suite *CompileTestSuite) TestMorpheRepoToGoPSQL() {
	workingDirPath := filepath.Join(suite.TestDirPath, "working")
	suite.Nil(os.Mkdir(workingDirPath, 0755))
	defer os.RemoveAll(workingDirPath)

	config := cfg.CompileConfig{
		MorpheLoadRegistryConfig: rcfg.MorpheLoadRegistryConfig{
			RegistryEnumsDirPath:      suite.EnumsDirPath,
			RegistryModelsDirPath:     suite.ModelsDirPath,
			RegistryStructuresDirPath: suite.StructuresDirPath,
			RegistryEntitiesDirPath:   suite.EntitiesDirPath,
		},
		RepoInputDirPath: suite.RepoDirPath,
		OutputDirPath:    workingDirPath,
		ModelsConfig: cfg.ModelsConfig{
			PackagePath: "github.com/test/app/internal/types/models",
		},
		RepoConfig: cfg.RepoConfig{
			PackagePath: "github.com/test/app/internal/database/repo",
		},
	}

	compileErr := compile.MorpheRepoToGoPSQL(config)

	suite.NoError(compileErr)

	// Verify organization_repository.go
	orgPath := filepath.Join(workingDirPath, "organization_repository.go")
	gtOrgPath := filepath.Join(suite.TestGroundTruthDirPath, "organization_repository.go")
	suite.FileExists(orgPath)
	suite.FileEquals(orgPath, gtOrgPath)

	// Verify project_repository.go
	projectPath := filepath.Join(workingDirPath, "project_repository.go")
	gtProjectPath := filepath.Join(suite.TestGroundTruthDirPath, "project_repository.go")
	suite.FileExists(projectPath)
	suite.FileEquals(projectPath, gtProjectPath)

	// Verify task_repository.go
	taskPath := filepath.Join(workingDirPath, "task_repository.go")
	gtTaskPath := filepath.Join(suite.TestGroundTruthDirPath, "task_repository.go")
	suite.FileExists(taskPath)
	suite.FileEquals(taskPath, gtTaskPath)
}

func (suite *CompileTestSuite) TestMorpheRepoToGoPSQL_MissingModel() {
	workingDirPath := filepath.Join(suite.TestDirPath, "working-missing-model")
	suite.Nil(os.Mkdir(workingDirPath, 0755))
	defer os.RemoveAll(workingDirPath)

	// Create a temp repo dir with a .repo referencing a nonexistent model
	tmpRepoDir := filepath.Join(suite.TestDirPath, "working-bad-repo")
	suite.Nil(os.Mkdir(tmpRepoDir, 0755))
	defer os.RemoveAll(tmpRepoDir)

	badRepo := `name: NonexistentRepository
model: Nonexistent

identifiers:
  primary:
    fields:
      - name: ID
        type: UUID

filters: []

operations:
  list: true
  get: true
  create: true
  update: true
  delete: true
`
	suite.Nil(os.WriteFile(filepath.Join(tmpRepoDir, "nonexistent.repo"), []byte(badRepo), 0644))

	config := cfg.CompileConfig{
		MorpheLoadRegistryConfig: rcfg.MorpheLoadRegistryConfig{
			RegistryEnumsDirPath:      suite.EnumsDirPath,
			RegistryModelsDirPath:     suite.ModelsDirPath,
			RegistryStructuresDirPath: suite.StructuresDirPath,
			RegistryEntitiesDirPath:   suite.EntitiesDirPath,
		},
		RepoInputDirPath: tmpRepoDir,
		OutputDirPath:    workingDirPath,
		ModelsConfig: cfg.ModelsConfig{
			PackagePath: "github.com/test/app/internal/types/models",
		},
		RepoConfig: cfg.RepoConfig{
			PackagePath: "github.com/test/app/internal/database/repo",
		},
	}

	compileErr := compile.MorpheRepoToGoPSQL(config)

	suite.Error(compileErr)
	suite.Contains(compileErr.Error(), "Nonexistent")
}
