package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/test/app/internal/types/models"
)

// ProjectRepository implements the ProjectRepository interface using PostgreSQL.
type ProjectRepository struct {
	pool *pgxpool.Pool
}

// NewProjectRepository creates a new ProjectRepository.
func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

// GetAll retrieves all Project records with optional filters.
func (r *ProjectRepository) GetAll(ctx context.Context, organizationID *string) ([]models.Project, error) {
	query := `SELECT code, description, id, name, organization_id FROM projects`

	var conditions []string
	var args []interface{}
	paramIdx := 1

	if organizationID != nil {
		conditions = append(conditions, fmt.Sprintf("organization_id = $%d", paramIdx))
		args = append(args, *organizationID)
		paramIdx++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY id"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Project
	for rows.Next() {
		var m models.Project
		err := rows.Scan(&m.Code, &m.Description, &m.ID, &m.Name, &m.OrganizationID)
		if err != nil {
			return nil, err
		}
		results = append(results, m)
	}

	return results, rows.Err()
}

// GetByID retrieves a Project by primary.
func (r *ProjectRepository) GetByID(ctx context.Context, id string) (*models.Project, error) {
	query := `SELECT code, description, id, name, organization_id FROM projects WHERE id = $1`

	var m models.Project
	err := r.pool.QueryRow(ctx, query, id).Scan(&m.Code, &m.Description, &m.ID, &m.Name, &m.OrganizationID)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// GetByCode retrieves a Project by code.
func (r *ProjectRepository) GetByCode(ctx context.Context, code string) (*models.Project, error) {
	query := `SELECT code, description, id, name, organization_id FROM projects WHERE code = $1`

	var m models.Project
	err := r.pool.QueryRow(ctx, query, code).Scan(&m.Code, &m.Description, &m.ID, &m.Name, &m.OrganizationID)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Create inserts a new Project record.
func (r *ProjectRepository) Create(ctx context.Context, input *models.Project) (*models.Project, error) {
	query := `INSERT INTO projects (code, description, id, name, organization_id) VALUES ($1, $2, $3, $4, $5) RETURNING code, description, id, name, organization_id`

	var m models.Project
	err := r.pool.QueryRow(ctx, query, input.Code, input.Description, input.ID, input.Name, input.OrganizationID).Scan(&m.Code, &m.Description, &m.ID, &m.Name, &m.OrganizationID)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Update modifies an existing Project record.
func (r *ProjectRepository) Update(ctx context.Context, id string, input *models.Project) (*models.Project, error) {
	query := `UPDATE projects SET code = $2, description = $3, name = $4, organization_id = $5 WHERE id = $1 RETURNING code, description, id, name, organization_id`

	var m models.Project
	err := r.pool.QueryRow(ctx, query, id, input.Code, input.Description, input.Name, input.OrganizationID).Scan(&m.Code, &m.Description, &m.ID, &m.Name, &m.OrganizationID)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Delete removes a Project record by ID.
func (r *ProjectRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM projects WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id)
	return err
}

// Query finds Project records matching the non-zero fields of the example struct.
// Non-nil relation pointers trigger JOINs; populated fields on relations add WHERE conditions.
func (r *ProjectRepository) Query(ctx context.Context, example *models.Project) ([]models.Project, error) {
	selectCols := []string{"p.code", "p.description", "p.id", "p.name", "p.organization_id"}
	joins := []string{}
	conditions := []string{}
	args := []interface{}{}
	paramIdx := 1

	if example.Code != "" {
		conditions = append(conditions, fmt.Sprintf("p.code = $%d", paramIdx))
		args = append(args, example.Code)
		paramIdx++
	}
	if example.Description != "" {
		conditions = append(conditions, fmt.Sprintf("p.description = $%d", paramIdx))
		args = append(args, example.Description)
		paramIdx++
	}
	if example.ID != "" {
		conditions = append(conditions, fmt.Sprintf("p.id = $%d", paramIdx))
		args = append(args, example.ID)
		paramIdx++
	}
	if example.Name != "" {
		conditions = append(conditions, fmt.Sprintf("p.name = $%d", paramIdx))
		args = append(args, example.Name)
		paramIdx++
	}
	if example.OrganizationID != nil && *example.OrganizationID != "" {
		conditions = append(conditions, fmt.Sprintf("p.organization_id = $%d", paramIdx))
		args = append(args, *example.OrganizationID)
		paramIdx++
	}

	scanOrganization := false
	if example.Organization != nil {
		scanOrganization = true
		joins = append(joins, "JOIN organizations o ON p.organization_id = o.id")
		selectCols = append(selectCols, "o.code", "o.id", "o.name")
		if example.Organization.Code != "" {
			conditions = append(conditions, fmt.Sprintf("o.code = $%d", paramIdx))
			args = append(args, example.Organization.Code)
			paramIdx++
		}
		if example.Organization.ID != "" {
			conditions = append(conditions, fmt.Sprintf("o.id = $%d", paramIdx))
			args = append(args, example.Organization.ID)
			paramIdx++
		}
		if example.Organization.Name != "" {
			conditions = append(conditions, fmt.Sprintf("o.name = $%d", paramIdx))
			args = append(args, example.Organization.Name)
			paramIdx++
		}
	}

	query := "SELECT " + strings.Join(selectCols, ", ") + " FROM projects p"
	for _, j := range joins {
		query += " " + j
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY p.id"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Project
	for rows.Next() {
		var m models.Project
		scanDest := []interface{}{&m.Code, &m.Description, &m.ID, &m.Name, &m.OrganizationID}

		var relOrganization models.Organization
		if scanOrganization {
			scanDest = append(scanDest, &relOrganization.Code, &relOrganization.ID, &relOrganization.Name)
		}

		if err := rows.Scan(scanDest...); err != nil {
			return nil, err
		}

		if scanOrganization {
			m.Organization = &relOrganization
		}

		results = append(results, m)
	}

	return results, rows.Err()
}

// QueryOne finds exactly one Project record matching the example.
// Returns pgx.ErrNoRows if no match, error if more than one match.
func (r *ProjectRepository) QueryOne(ctx context.Context, example *models.Project) (*models.Project, error) {
	results, err := r.Query(ctx, example)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, pgx.ErrNoRows
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("QueryOne Project: expected 1 result, got %d", len(results))
	}
	return &results[0], nil
}
