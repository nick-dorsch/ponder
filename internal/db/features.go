package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nick-dorsch/ponder/pkg/models"
)

func (db *DB) CreateFeature(ctx context.Context, f *models.Feature) error {
	if err := db.createFeature(ctx, db.DB, f); err != nil {
		return err
	}

	db.triggerChange(ctx)
	return nil
}

func (db *DB) GetFeature(ctx context.Context, id string) (*models.Feature, error) {
	query := `
		SELECT id, name, description, specification, created_at, updated_at
		FROM features
		WHERE id = ?
	`
	f := &models.Feature{}
	err := db.QueryRowContext(ctx, query, id).Scan(
		&f.ID, &f.Name, &f.Description, &f.Specification, &f.CreatedAt, &f.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get feature: %w", err)
	}

	return f, nil
}

func (db *DB) GetFeatureByName(ctx context.Context, name string) (*models.Feature, error) {
	return db.getFeatureByName(ctx, db.DB, name)
}

func (db *DB) getFeatureByName(ctx context.Context, exec executor, name string) (*models.Feature, error) {
	query := `
		SELECT id, name, description, specification, created_at, updated_at
		FROM features
		WHERE name = ?
	`
	f := &models.Feature{}
	err := exec.QueryRowContext(ctx, query, name).Scan(
		&f.ID, &f.Name, &f.Description, &f.Specification, &f.CreatedAt, &f.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get feature by name: %w", err)
	}

	return f, nil
}

func (db *DB) ListFeatures(ctx context.Context) ([]*models.Feature, error) {
	query := `
		SELECT id, name, description, specification, created_at, updated_at
		FROM features
		ORDER BY created_at DESC
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list features: %w", err)
	}
	defer rows.Close()

	var features []*models.Feature
	for rows.Next() {
		f := &models.Feature{}
		err := rows.Scan(
			&f.ID, &f.Name, &f.Description, &f.Specification, &f.CreatedAt, &f.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan feature: %w", err)
		}
		features = append(features, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return features, nil
}

func (db *DB) UpdateFeature(ctx context.Context, f *models.Feature) error {
	query := `
		UPDATE features
		SET name = ?, description = ?, specification = ?
		WHERE id = ?
		RETURNING updated_at
	`
	err := db.QueryRowContext(ctx, query, f.Name, f.Description, f.Specification, f.ID).Scan(&f.UpdatedAt)
	if err == sql.ErrNoRows {
		return fmt.Errorf("feature not found: %s", f.ID)
	}
	if err != nil {
		return fmt.Errorf("failed to update feature: %w", err)
	}

	db.triggerChange(ctx)
	return nil
}

func (db *DB) DeleteFeature(ctx context.Context, id string) error {
	query := `DELETE FROM features WHERE id = ?`
	res, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete feature: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("feature not found: %s", id)
	}

	db.triggerChange(ctx)
	return nil
}
