package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/test/app/internal/types/models"
)

// OrganizationRepository implements the OrganizationRepository interface using PostgreSQL.
type OrganizationRepository struct {
	pool *pgxpool.Pool
}

// NewOrganizationRepository creates a new OrganizationRepository.
func NewOrganizationRepository(pool *pgxpool.Pool) *OrganizationRepository {
	return &OrganizationRepository{pool: pool}
}

// GetAll retrieves all Organization records with optional filters.
func (r *OrganizationRepository) GetAll(ctx context.Context) ([]models.Organization, error) {
	query := `SELECT code, id, name FROM organizations`
	query += " ORDER BY id"

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Organization
	for rows.Next() {
		var m models.Organization
		err := rows.Scan(&m.Code, &m.ID, &m.Name)
		if err != nil {
			return nil, err
		}
		results = append(results, m)
	}

	return results, rows.Err()
}

// GetByID retrieves a Organization by primary.
func (r *OrganizationRepository) GetByID(ctx context.Context, id string) (*models.Organization, error) {
	query := `SELECT code, id, name FROM organizations WHERE id = $1`

	var m models.Organization
	err := r.pool.QueryRow(ctx, query, id).Scan(&m.Code, &m.ID, &m.Name)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// GetByCode retrieves a Organization by code.
func (r *OrganizationRepository) GetByCode(ctx context.Context, code string) (*models.Organization, error) {
	query := `SELECT code, id, name FROM organizations WHERE code = $1`

	var m models.Organization
	err := r.pool.QueryRow(ctx, query, code).Scan(&m.Code, &m.ID, &m.Name)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Create inserts a new Organization record.
func (r *OrganizationRepository) Create(ctx context.Context, input *models.Organization) (*models.Organization, error) {
	query := `INSERT INTO organizations (code, id, name) VALUES ($1, $2, $3) RETURNING code, id, name`

	var m models.Organization
	err := r.pool.QueryRow(ctx, query, input.Code, input.ID, input.Name).Scan(&m.Code, &m.ID, &m.Name)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Update modifies an existing Organization record.
func (r *OrganizationRepository) Update(ctx context.Context, id string, input *models.Organization) (*models.Organization, error) {
	query := `UPDATE organizations SET code = $2, name = $3 WHERE id = $1 RETURNING code, id, name`

	var m models.Organization
	err := r.pool.QueryRow(ctx, query, id, input.Code, input.Name).Scan(&m.Code, &m.ID, &m.Name)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Delete removes a Organization record by ID.
func (r *OrganizationRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM organizations WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id)
	return err
}

// Query finds Organization records matching the non-zero fields of the example struct.
// Non-nil relation pointers trigger JOINs; populated fields on relations add WHERE conditions.
func (r *OrganizationRepository) Query(ctx context.Context, example *models.Organization) ([]models.Organization, error) {
	selectCols := []string{"o.code", "o.id", "o.name"}
	conditions := []string{}
	args := []interface{}{}
	paramIdx := 1

	if example.Code != "" {
		conditions = append(conditions, fmt.Sprintf("o.code = $%d", paramIdx))
		args = append(args, example.Code)
		paramIdx++
	}
	if example.ID != "" {
		conditions = append(conditions, fmt.Sprintf("o.id = $%d", paramIdx))
		args = append(args, example.ID)
		paramIdx++
	}
	if example.Name != "" {
		conditions = append(conditions, fmt.Sprintf("o.name = $%d", paramIdx))
		args = append(args, example.Name)
		paramIdx++
	}

	query := "SELECT " + strings.Join(selectCols, ", ") + " FROM organizations o"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY o.id"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Organization
	for rows.Next() {
		var m models.Organization
		scanDest := []interface{}{&m.Code, &m.ID, &m.Name}

		if err := rows.Scan(scanDest...); err != nil {
			return nil, err
		}

		results = append(results, m)
	}

	return results, rows.Err()
}

// QueryOne finds exactly one Organization record matching the example.
// Returns pgx.ErrNoRows if no match, error if more than one match.
func (r *OrganizationRepository) QueryOne(ctx context.Context, example *models.Organization) (*models.Organization, error) {
	results, err := r.Query(ctx, example)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, pgx.ErrNoRows
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("QueryOne Organization: expected 1 result, got %d", len(results))
	}
	return &results[0], nil
}
