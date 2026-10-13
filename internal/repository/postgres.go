package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgUniqueViolation — код ошибки PostgreSQL при нарушении уникальности (PK).
const pgUniqueViolation = "23505"

// pgForeignKeyViolation — код ошибки PostgreSQL при нарушении внешнего ключа
// (ON DELETE RESTRICT): на карточку ссылается оборудование.
const pgForeignKeyViolation = "23503"

// CardRepo — реализация CardRepository поверх pgxpool.
type CardRepo struct {
	pool *pgxpool.Pool
}

// NewCardRepo создаёт репозиторий карточек на переданном пуле.
func NewCardRepo(pool *pgxpool.Pool) *CardRepo {
	return &CardRepo{pool: pool}
}

// Compile-time проверка, что CardRepo реализует CardRepository.
var _ CardRepository = (*CardRepo)(nil)

// cardColumns — список колонок для чтения карточки (единый для всех SELECT).
const cardColumns = `number, full_name, is_active, created_at, updated_at, deleted_at`

func (r *CardRepo) Create(ctx context.Context, card *domain.Card) error {
	_, err := r.pool.Exec(ctx, `
        INSERT INTO cards (number, full_name, is_active, created_at, updated_at, deleted_at)
        VALUES ($1, $2, $3, $4, $5, $6)`,
		card.Number, card.FullName, card.IsActive, card.CreatedAt, card.UpdatedAt, card.DeletedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("%w: номер %d", domain.ErrAlreadyExists, card.Number)
		}
		return fmt.Errorf("insert card %d: %w", card.Number, err)
	}
	return nil
}

func (r *CardRepo) GetByNumber(ctx context.Context, number int) (*domain.Card, error) {
	row := r.pool.QueryRow(ctx, `
        SELECT `+cardColumns+` FROM cards WHERE number = $1`, number)

	card, err := scanCard(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: номер %d", domain.ErrNotFound, number)
		}
		return nil, fmt.Errorf("select card %d: %w", number, err)
	}
	return card, nil
}

func (r *CardRepo) ListActive(ctx context.Context) ([]*domain.Card, error) {
	return r.list(ctx, true)
}

func (r *CardRepo) ListInactive(ctx context.Context) ([]*domain.Card, error) {
	return r.list(ctx, false)
}

// list загружает карточки с фильтром по активности, отсортированные по номеру.
func (r *CardRepo) list(ctx context.Context, active bool) ([]*domain.Card, error) {
	rows, err := r.pool.Query(ctx, `
        SELECT `+cardColumns+` FROM cards
        WHERE is_active = $1
        ORDER BY number`, active)
	if err != nil {
		return nil, fmt.Errorf("query cards (active=%v): %w", active, err)
	}
	defer rows.Close()

	cards := make([]*domain.Card, 0)
	for rows.Next() {
		card, err := scanCard(rows)
		if err != nil {
			return nil, fmt.Errorf("scan card: %w", err)
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cards: %w", err)
	}
	return cards, nil
}

func (r *CardRepo) Exists(ctx context.Context, number int) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM cards WHERE number = $1)`, number,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check card %d exists: %w", number, err)
	}
	return exists, nil
}

func (r *CardRepo) UpdateFullName(ctx context.Context, number int, fullName string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE cards SET full_name = $1 WHERE number = $2`,
		fullName, number,
	)
	if err != nil {
		return fmt.Errorf("update card %d: %w", number, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: номер %d", domain.ErrNotFound, number)
	}
	return nil
}

func (r *CardRepo) Deactivate(ctx context.Context, number int) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE cards
        SET is_active = false,
            deleted_at = NOW(),
            updated_at = NOW()
        WHERE number = $1 AND is_active = true`,
		number,
	)
	if err != nil {
		return fmt.Errorf("deactivate card %d: %w", number, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: номер %d", domain.ErrNotFound, number)
	}
	return nil
}

func (r *CardRepo) Delete(ctx context.Context, number int) error {
	tag, err := r.pool.Exec(ctx, `
        DELETE FROM cards WHERE number = $1`, number)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return fmt.Errorf("%w: номер %d", domain.ErrInUse, number)
		}
		return fmt.Errorf("delete card %d: %w", number, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: номер %d", domain.ErrNotFound, number)
	}
	return nil
}

// cardScanner — то, что умеют и *pgx.Row, и pgx.Rows.
type cardScanner interface {
	Scan(dest ...any) error
}

// scanCard читает одну строку cards в доменную модель.
func scanCard(row cardScanner) (*domain.Card, error) {
	var c domain.Card
	var deletedAt *time.Time
	if err := row.Scan(
		&c.Number, &c.FullName, &c.IsActive,
		&c.CreatedAt, &c.UpdatedAt, &deletedAt,
	); err != nil {
		return nil, err
	}
	c.DeletedAt = deletedAt
	return &c, nil
}
