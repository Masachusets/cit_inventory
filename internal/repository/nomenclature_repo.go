package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NomenclatureRepo — реализация NomenclatureRepository поверх pgxpool.
type NomenclatureRepo struct {
	pool *pgxpool.Pool
}

// NewNomenclatureRepo создаёт репозиторий номенклатуры на переданном пуле.
func NewNomenclatureRepo(pool *pgxpool.Pool) *NomenclatureRepo {
	return &NomenclatureRepo{pool: pool}
}

// Compile-time проверка, что NomenclatureRepo реализует NomenclatureRepository.
var _ NomenclatureRepository = (*NomenclatureRepo)(nil)

// nomenclatureColumns — список колонок для чтения номенклатуры.
const nomenclatureColumns = `id, code, name`

func (r *NomenclatureRepo) GetByID(ctx context.Context, id int) (*domain.Nomenclature, error) {
	row := r.pool.QueryRow(ctx, `
        SELECT `+nomenclatureColumns+` FROM nomenclatures WHERE id = $1`, id)

	var n domain.Nomenclature
	if err := row.Scan(&n.ID, &n.Code, &n.Name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: id %d", domain.ErrNotFound, id)
		}
		return nil, fmt.Errorf("select nomenclature %d: %w", id, err)
	}
	return &n, nil
}

func (r *NomenclatureRepo) List(ctx context.Context, filter domain.NomenclatureFilter) ([]*domain.Nomenclature, error) {
	query := `SELECT ` + nomenclatureColumns + ` FROM nomenclatures`
	var (
		conds []string
		args  []any
	)
	if filter.Code != nil {
		args = append(args, *filter.Code)
		conds = append(conds, fmt.Sprintf("code ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if filter.Name != nil {
		args = append(args, *filter.Name)
		conds = append(conds, fmt.Sprintf("name ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY id"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query nomenclatures: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Nomenclature, 0)
	for rows.Next() {
		var n domain.Nomenclature
		if err := rows.Scan(&n.ID, &n.Code, &n.Name); err != nil {
			return nil, fmt.Errorf("scan nomenclature: %w", err)
		}
		items = append(items, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nomenclatures: %w", err)
	}
	return items, nil
}
