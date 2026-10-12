package importer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ImportCards загружает карточки учёта в таблицу cards одной транзакцией.
// Импорт одноразовый (без синхронизации): повторный запуск с теми же
// номерами завершится ошибкой UNIQUE — это ожидаемо.
// Возвращает количество вставленных строк.
func ImportCards(ctx context.Context, pool *pgxpool.Pool, cards []Card) (int, error) {
	if len(cards) == 0 {
		return 0, nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // откат после Commit безвреден

	rows := make([][]any, 0, len(cards))
	for _, c := range cards {
		rows = append(rows, []any{c.Number, c.FullName, c.Date})
	}

	tag, err := tx.CopyFrom(ctx,
		pgx.Identifier{"cards"},
		[]string{"number", "full_name", "created_at"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return 0, fmt.Errorf("copy cards: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return int(tag), nil
}
