package compile_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kalo-build/morphe-go/pkg/yaml"
	"github.com/kalo-build/plugin-morpherepo-go-psql/pkg/compile"
	"github.com/kalo-build/plugin-morpherepo-go-psql/pkg/compile/cfg"
	"github.com/kalo-build/plugin-morpherepo-go/pkg/repo"
)

type GenerateRepositoryTestSuite struct {
	suite.Suite

	Config cfg.CompileConfig
}

func TestGenerateRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(GenerateRepositoryTestSuite))
}

func (suite *GenerateRepositoryTestSuite) SetupTest() {
	suite.Config = cfg.CompileConfig{
		ModelsConfig: cfg.ModelsConfig{
			PackagePath: "github.com/test/app/internal/types/models",
		},
		RepoConfig: cfg.RepoConfig{
			PackagePath: "github.com/test/app/internal/database/repo",
		},
	}
}

func (suite *GenerateRepositoryTestSuite) TestGenerateRepository_SimpleModel() {
	spec := repo.RepoSpec{
		Name:  "OrganizationRepository",
		Model: "Organization",
		Identifiers: map[string]repo.Identifier{
			"primary": {Fields: []repo.IdentifierField{{Name: "ID", Type: "UUID"}}},
			"code":    {Fields: []repo.IdentifierField{{Name: "Code", Type: "String"}}},
		},
		Filters: []repo.Filter{},
		Operations: repo.Operations{
			List: true, Get: true, Create: true, Update: true, Delete: true,
		},
	}

	model := yaml.Model{
		Name: "Organization",
		Fields: map[string]yaml.ModelField{
			"ID":   {Type: "UUID"},
			"Code": {Type: "String"},
			"Name": {Type: "String"},
		},
		Related: map[string]yaml.ModelRelation{},
	}

	allModels := map[string]yaml.Model{"Organization": model}
	code := compile.GenerateRepository(spec, model, allModels, nil, suite.Config)

	suite.Contains(code, "package repo")

	// Verify struct
	suite.Contains(code, "type OrganizationRepository struct {")
	suite.Contains(code, "pool *pgxpool.Pool")

	// Verify constructor
	suite.Contains(code, "func NewOrganizationRepository(pool *pgxpool.Pool) *OrganizationRepository")

	// Verify GetAll (no filters)
	suite.Contains(code, "func (r *OrganizationRepository) GetAll(ctx context.Context) ([]models.Organization, error)")
	suite.Contains(code, "SELECT code, id, name FROM organizations")

	// Verify GetByID
	suite.Contains(code, "func (r *OrganizationRepository) GetByID(ctx context.Context, id string)")
	suite.Contains(code, "WHERE id = $1")

	// Verify GetByCode
	suite.Contains(code, "func (r *OrganizationRepository) GetByCode(ctx context.Context, code string)")
	suite.Contains(code, "WHERE code = $1")

	// Verify Create
	suite.Contains(code, "func (r *OrganizationRepository) Create(ctx context.Context, input *models.Organization)")
	suite.Contains(code, "INSERT INTO organizations")
	suite.Contains(code, "RETURNING")

	// Verify Update excludes ID
	suite.Contains(code, "func (r *OrganizationRepository) Update(ctx context.Context, id string, input *models.Organization)")
	suite.Contains(code, "UPDATE organizations SET")
	suite.Contains(code, "WHERE id = $1")

	// Verify Delete
	suite.Contains(code, "func (r *OrganizationRepository) Delete(ctx context.Context, id string) error")
	suite.Contains(code, "DELETE FROM organizations WHERE id = $1")

	// Verify Query
	suite.Contains(code, "func (r *OrganizationRepository) Query(ctx context.Context, example *models.Organization) ([]models.Organization, error)")
	suite.Contains(code, "FROM organizations o")
	suite.Contains(code, "example.Code != \"\"")
	suite.Contains(code, "example.ID != \"\"")
	suite.Contains(code, "example.Name != \"\"")

	// Verify QueryOne
	suite.Contains(code, "func (r *OrganizationRepository) QueryOne(ctx context.Context, example *models.Organization) (*models.Organization, error)")
	suite.Contains(code, "pgx.ErrNoRows")
}

func (suite *GenerateRepositoryTestSuite) TestGenerateRepository_ModelWithForOne() {
	spec := repo.RepoSpec{
		Name:  "ProjectRepository",
		Model: "Project",
		Identifiers: map[string]repo.Identifier{
			"primary": {Fields: []repo.IdentifierField{{Name: "ID", Type: "UUID"}}},
			"code":    {Fields: []repo.IdentifierField{{Name: "Code", Type: "String"}}},
		},
		Filters: []repo.Filter{
			{Name: "organizationID", Type: "UUID", Relation: "Organization"},
		},
		Operations: repo.Operations{
			List: true, Get: true, Create: true, Update: true, Delete: true,
		},
	}

	model := yaml.Model{
		Name: "Project",
		Fields: map[string]yaml.ModelField{
			"ID":          {Type: "UUID"},
			"Code":        {Type: "String"},
			"Name":        {Type: "String"},
			"Description": {Type: "String", Attributes: []string{"optional"}},
		},
		Related: map[string]yaml.ModelRelation{
			"Organization": {Type: "ForOne"},
		},
	}

	orgModel := yaml.Model{
		Name: "Organization",
		Fields: map[string]yaml.ModelField{
			"ID":   {Type: "UUID"},
			"Code": {Type: "String"},
			"Name": {Type: "String"},
		},
		Identifiers: map[string]yaml.ModelIdentifier{
			"primary": {Fields: []string{"ID"}},
		},
		Related: map[string]yaml.ModelRelation{},
	}
	allModels := map[string]yaml.Model{"Project": model, "Organization": orgModel}
	code := compile.GenerateRepository(spec, model, allModels, nil, suite.Config)

	// Verify imports include fmt and strings
	suite.Contains(code, "\"fmt\"")
	suite.Contains(code, "\"strings\"")

	// Verify GetAll with filter parameter
	suite.Contains(code, "func (r *ProjectRepository) GetAll(ctx context.Context, organizationID *string)")
	suite.Contains(code, "organization_id = $%d")
	suite.Contains(code, "if organizationID != nil")

	// Verify FK column included in SELECT
	suite.Contains(code, "organization_id")

	// Verify FK column included in Create
	suite.Contains(code, "input.OrganizationID")

	// Verify FK column included in Update
	suite.Contains(code, "organization_id = $")

	// Verify Query with JOIN for Organization relation
	suite.Contains(code, "func (r *ProjectRepository) Query(ctx context.Context, example *models.Project) ([]models.Project, error)")
	suite.Contains(code, "scanOrganization := false")
	suite.Contains(code, "example.Organization != nil")
	suite.Contains(code, "JOIN organizations o ON")
	suite.Contains(code, "m.Organization = &relOrganization")

	// Verify optional field uses pointer check
	suite.Contains(code, "example.Description != nil")
	suite.NotContains(code, "example.Description != \"\"")
	suite.Contains(code, "*example.Description")

	// Verify QueryOne
	suite.Contains(code, "func (r *ProjectRepository) QueryOne(ctx context.Context, example *models.Project) (*models.Project, error)")
}

func (suite *GenerateRepositoryTestSuite) TestGenerateRepository_OnlyPrimaryIdentifier() {
	spec := repo.RepoSpec{
		Name:  "TaskRepository",
		Model: "Task",
		Identifiers: map[string]repo.Identifier{
			"primary": {Fields: []repo.IdentifierField{{Name: "ID", Type: "UUID"}}},
		},
		Filters: []repo.Filter{
			{Name: "projectID", Type: "UUID", Relation: "Project"},
		},
		Operations: repo.Operations{
			List: true, Get: true, Create: true, Update: true, Delete: true,
		},
	}

	model := yaml.Model{
		Name: "Task",
		Fields: map[string]yaml.ModelField{
			"ID":     {Type: "UUID"},
			"Title":  {Type: "String"},
			"Status": {Type: "String"},
		},
		Related: map[string]yaml.ModelRelation{
			"Project": {Type: "ForOne", Attributes: []string{"optional"}},
		},
	}

	projectModel := yaml.Model{
		Name: "Project",
		Fields: map[string]yaml.ModelField{
			"ID":          {Type: "UUID"},
			"Code":        {Type: "String"},
			"Name":        {Type: "String"},
			"Description": {Type: "String", Attributes: []string{"optional"}},
		},
		Identifiers: map[string]yaml.ModelIdentifier{
			"primary": {Fields: []string{"ID"}},
		},
	}
	allModels := map[string]yaml.Model{"Task": model, "Project": projectModel}
	code := compile.GenerateRepository(spec, model, allModels, nil, suite.Config)

	// Only GetByID, no GetByCode
	suite.Contains(code, "func (r *TaskRepository) GetByID(ctx context.Context, id string)")
	suite.NotContains(code, "GetByCode")

	// Verify FK filter
	suite.Contains(code, "projectID *string")
	suite.Contains(code, "project_id = $%d")

	// Verify optional FK uses pointer check
	suite.Contains(code, "example.ProjectID != nil")
	suite.NotContains(code, "example.ProjectID != \"\"")
	suite.Contains(code, "*example.ProjectID")

	// Verify Query with Project relation
	suite.Contains(code, "scanProject := false")
	suite.Contains(code, "example.Project != nil")
	suite.Contains(code, "JOIN projects")

	// Verify optional field on joined relation uses pointer check
	suite.Contains(code, "example.Project.Description != nil")
	suite.NotContains(code, "example.Project.Description != \"\"")
	suite.Contains(code, "*example.Project.Description")
}

func (suite *GenerateRepositoryTestSuite) TestGenerateRepository_PartialOperations() {
	spec := repo.RepoSpec{
		Name:  "ReadOnlyRepository",
		Model: "Organization",
		Identifiers: map[string]repo.Identifier{
			"primary": {Fields: []repo.IdentifierField{{Name: "ID", Type: "UUID"}}},
		},
		Filters:    []repo.Filter{},
		Operations: repo.Operations{List: true, Get: true},
	}

	model := yaml.Model{
		Name: "Organization",
		Fields: map[string]yaml.ModelField{
			"ID":   {Type: "UUID"},
			"Name": {Type: "String"},
		},
		Related: map[string]yaml.ModelRelation{},
	}

	allModels := map[string]yaml.Model{"Organization": model}
	code := compile.GenerateRepository(spec, model, allModels, nil, suite.Config)

	// Should have GetAll and GetByID
	suite.Contains(code, "GetAll")
	suite.Contains(code, "GetByID")

	// Should NOT have Create, Update, Delete (but Query/QueryOne are always generated)
	suite.NotContains(code, "func (r *OrganizationRepository) Create")
	suite.NotContains(code, "func (r *OrganizationRepository) Update")
	suite.NotContains(code, "func (r *OrganizationRepository) Delete")
	suite.Contains(code, "func (r *OrganizationRepository) Query")
	suite.Contains(code, "func (r *OrganizationRepository) QueryOne")
}

func (suite *GenerateRepositoryTestSuite) TestGenerateRepository_EnumField() {
	spec := repo.RepoSpec{
		Name:  "TaskRepository",
		Model: "Task",
		Identifiers: map[string]repo.Identifier{
			"primary": {Fields: []repo.IdentifierField{{Name: "ID", Type: "UUID"}}},
		},
		Filters: []repo.Filter{
			{Name: "projectID", Type: "UUID", Relation: "Project"},
		},
		Operations: repo.Operations{
			List: true, Get: true, Create: true, Update: true, Delete: true,
		},
	}

	model := yaml.Model{
		Name: "Task",
		Fields: map[string]yaml.ModelField{
			"ID":     {Type: "UUID"},
			"Title":  {Type: "String"},
			"Status": {Type: "TaskStatus"},
		},
		Related: map[string]yaml.ModelRelation{
			"Project": {Type: "ForOne", Attributes: []string{"optional"}},
		},
	}

	allEnums := map[string]yaml.Enum{
		"TaskStatus": {Name: "TaskStatus", Type: "String", Entries: map[string]any{"Todo": "todo", "InProgress": "in_progress", "Done": "done"}},
	}

	projectModel := yaml.Model{
		Name: "Project",
		Fields: map[string]yaml.ModelField{
			"ID":   {Type: "UUID"},
			"Name": {Type: "String"},
		},
		Identifiers: map[string]yaml.ModelIdentifier{
			"primary": {Fields: []string{"ID"}},
		},
	}
	allModels := map[string]yaml.Model{"Task": model, "Project": projectModel}
	code := compile.GenerateRepository(spec, model, allModels, allEnums, suite.Config)

	// GetAll SELECT should resolve enum via subquery
	suite.Contains(code, "(SELECT value FROM task_statuses WHERE id = status_id)")
	suite.NotContains(code, "SELECT id, status, title")

	// GetByID should also use subquery
	suite.Contains(code, "WHERE id = $1")

	// Create should use subquery for the FK lookup and RETURNING
	suite.Contains(code, "(SELECT id FROM task_statuses WHERE value = $")
	suite.Contains(code, "RETURNING")

	// Update SET should use subquery
	suite.Contains(code, "status_id = (SELECT id FROM task_statuses WHERE value = $")

	// Scan still uses the Go field name
	suite.Contains(code, "&m.Status")

	// Query method should handle enum in selectCols and conditions
	suite.Contains(code, "(SELECT value FROM task_statuses WHERE id = t.status_id)")
	suite.Contains(code, "(SELECT id FROM task_statuses WHERE value = $%d)")
}

func (suite *GenerateRepositoryTestSuite) TestGenerateRepository_ForOnePoly() {
	spec := repo.RepoSpec{
		Name:  "PluginRepository",
		Model: "Plugin",
		Identifiers: map[string]repo.Identifier{
			"primary": {Fields: []repo.IdentifierField{{Name: "ID", Type: "UUID"}}},
		},
		Filters: []repo.Filter{
			{Name: "ownerID", Type: "UUID", Relation: "Owner"},
		},
		Operations: repo.Operations{List: true, Get: true, Create: true, Update: true, Delete: true},
	}

	model := yaml.Model{
		Name: "Plugin",
		Fields: map[string]yaml.ModelField{
			"ID":   {Type: "UUID"},
			"Name": {Type: "String"},
		},
		Related: map[string]yaml.ModelRelation{
			"Owner": {Type: "ForOnePoly", Through: "Owner", For: []string{"User", "Organization"}},
		},
	}

	allModels := map[string]yaml.Model{"Plugin": model}
	code := compile.GenerateRepository(spec, model, allModels, nil, suite.Config)

	// Verify polymorphic columns
	suite.Contains(code, "owner_id")
	suite.Contains(code, "owner_type")
	suite.Contains(code, "&m.OwnerID")
	suite.Contains(code, "&m.OwnerType")

	// Verify filter uses owner_id
	suite.Contains(code, "owner_id = $%d")

	// OwnerID and OwnerType should NOT be pointer types (poly fields are never optional)
	// The scan should reference them directly as non-pointer fields
	suite.True(strings.Contains(code, "input.OwnerID"))
	suite.True(strings.Contains(code, "input.OwnerType"))
}
