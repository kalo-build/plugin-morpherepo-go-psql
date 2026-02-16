package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/test/app/internal/types/models"
)

// TaskRepository implements the TaskRepository interface using PostgreSQL.
type TaskRepository struct {
	pool *pgxpool.Pool
}

// NewTaskRepository creates a new TaskRepository.
func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{pool: pool}
}

// GetAll retrieves all Task records with optional filters.
func (r *TaskRepository) GetAll(ctx context.Context, projectID *string) ([]models.Task, error) {
	query := `SELECT id, status, title, project_id FROM tasks`

	var conditions []string
	var args []interface{}
	paramIdx := 1

	if projectID != nil {
		conditions = append(conditions, fmt.Sprintf("project_id = $%d", paramIdx))
		args = append(args, *projectID)
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

	var results []models.Task
	for rows.Next() {
		var m models.Task
		err := rows.Scan(&m.ID, &m.Status, &m.Title, &m.ProjectID)
		if err != nil {
			return nil, err
		}
		results = append(results, m)
	}

	return results, rows.Err()
}

// GetByID retrieves a Task by primary.
func (r *TaskRepository) GetByID(ctx context.Context, id string) (*models.Task, error) {
	query := `SELECT id, status, title, project_id FROM tasks WHERE id = $1`

	var m models.Task
	err := r.pool.QueryRow(ctx, query, id).Scan(&m.ID, &m.Status, &m.Title, &m.ProjectID)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Create inserts a new Task record.
func (r *TaskRepository) Create(ctx context.Context, input *models.Task) (*models.Task, error) {
	query := `INSERT INTO tasks (id, status, title, project_id) VALUES ($1, $2, $3, $4) RETURNING id, status, title, project_id`

	var m models.Task
	err := r.pool.QueryRow(ctx, query, input.ID, input.Status, input.Title, input.ProjectID).Scan(&m.ID, &m.Status, &m.Title, &m.ProjectID)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Update modifies an existing Task record.
func (r *TaskRepository) Update(ctx context.Context, id string, input *models.Task) (*models.Task, error) {
	query := `UPDATE tasks SET status = $2, title = $3, project_id = $4 WHERE id = $1 RETURNING id, status, title, project_id`

	var m models.Task
	err := r.pool.QueryRow(ctx, query, id, input.Status, input.Title, input.ProjectID).Scan(&m.ID, &m.Status, &m.Title, &m.ProjectID)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Delete removes a Task record by ID.
func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM tasks WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id)
	return err
}

// Query finds Task records matching the non-zero fields of the example struct.
// Non-nil relation pointers trigger JOINs; populated fields on relations add WHERE conditions.
func (r *TaskRepository) Query(ctx context.Context, example *models.Task) ([]models.Task, error) {
	selectCols := []string{"t.id", "t.status", "t.title", "t.project_id"}
	joins := []string{}
	conditions := []string{}
	args := []interface{}{}
	paramIdx := 1

	if example.ID != "" {
		conditions = append(conditions, fmt.Sprintf("t.id = $%d", paramIdx))
		args = append(args, example.ID)
		paramIdx++
	}
	if example.Status != "" {
		conditions = append(conditions, fmt.Sprintf("t.status = $%d", paramIdx))
		args = append(args, example.Status)
		paramIdx++
	}
	if example.Title != "" {
		conditions = append(conditions, fmt.Sprintf("t.title = $%d", paramIdx))
		args = append(args, example.Title)
		paramIdx++
	}
	if example.ProjectID != nil && *example.ProjectID != "" {
		conditions = append(conditions, fmt.Sprintf("t.project_id = $%d", paramIdx))
		args = append(args, *example.ProjectID)
		paramIdx++
	}

	scanProject := false
	if example.Project != nil {
		scanProject = true
		joins = append(joins, "JOIN projects p ON t.project_id = p.id")
		selectCols = append(selectCols, "p.code", "p.description", "p.id", "p.name")
		if example.Project.Code != "" {
			conditions = append(conditions, fmt.Sprintf("p.code = $%d", paramIdx))
			args = append(args, example.Project.Code)
			paramIdx++
		}
		if example.Project.Description != "" {
			conditions = append(conditions, fmt.Sprintf("p.description = $%d", paramIdx))
			args = append(args, example.Project.Description)
			paramIdx++
		}
		if example.Project.ID != "" {
			conditions = append(conditions, fmt.Sprintf("p.id = $%d", paramIdx))
			args = append(args, example.Project.ID)
			paramIdx++
		}
		if example.Project.Name != "" {
			conditions = append(conditions, fmt.Sprintf("p.name = $%d", paramIdx))
			args = append(args, example.Project.Name)
			paramIdx++
		}
	}

	query := "SELECT " + strings.Join(selectCols, ", ") + " FROM tasks t"
	for _, j := range joins {
		query += " " + j
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY t.id"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Task
	for rows.Next() {
		var m models.Task
		scanDest := []interface{}{&m.ID, &m.Status, &m.Title, &m.ProjectID}

		var relProject models.Project
		if scanProject {
			scanDest = append(scanDest, &relProject.Code, &relProject.Description, &relProject.ID, &relProject.Name)
		}

		if err := rows.Scan(scanDest...); err != nil {
			return nil, err
		}

		if scanProject {
			m.Project = &relProject
		}

		results = append(results, m)
	}

	return results, rows.Err()
}

// QueryOne finds exactly one Task record matching the example.
// Returns pgx.ErrNoRows if no match, error if more than one match.
func (r *TaskRepository) QueryOne(ctx context.Context, example *models.Task) (*models.Task, error) {
	results, err := r.Query(ctx, example)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, pgx.ErrNoRows
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("QueryOne Task: expected 1 result, got %d", len(results))
	}
	return &results[0], nil
}
