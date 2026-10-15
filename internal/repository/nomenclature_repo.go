package repository

import (
	"context"
	"errors"
	"fmt"

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

// nomenclaturesListQuery собирает SELECT списка номенклатуры (чистая функция —
// тестируется без БД).
func nomenclaturesListQuery(filter domain.NomenclatureFilter) (string, []any) {
	var f filterBuilder
	f.ilike("code", filter.Code)
	f.ilike("name", filter.Name)
	return f.where("SELECT "+nomenclatureColumns+" FROM nomenclatures") + " ORDER BY id", f.args
}

func (r *NomenclatureRepo) List(ctx context.Context, filter domain.NomenclatureFilter) ([]*domain.Nomenclature, error) {
	query, args := nomenclaturesListQuery(filter)

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
