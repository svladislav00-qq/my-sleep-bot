package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/svladilsav00-qq/my-sleep-bot/storage"
)

type Storage struct {
	db *sql.DB
}

func New(dsn string) (*Storage, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("can't open database: %w", err)
	}

	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("can't connect to database: %w", err)
	}

	return &Storage{db: db}, nil
}

func (s *Storage) SaveRecord(ctx context.Context, record *storage.SleepRecord) error {
	q := `INSERT INTO records (user_id, fell_sleep, woke_up) 
	VALUES ($1, $2, $3, $4)
	RETURNING id
	`

	if _, err := s.db.ExecContext(ctx, q, record.ID, record.UserID, record.FellAsleepAT, record.WokeUpAt); err != nil {
		return fmt.Errorf("can't save record: %w", err)
	}

	return nil
}

func (s *Storage) EditRecord(ctx context.Context, record *storage.SleepRecord) error {
	q := `
		UPDATE records
		SET fell_sleep = $1, woke_up = $2 
		WHERE id = $3 AND user_id = $4 
		`

	res, err := s.db.ExecContext(ctx, q, record.FellAsleepAT, record.WokeUpAt, record.ID, record.UserID)
	if err != nil {
		return fmt.Errorf("can't edit record: %w", err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("can't check edited record: %w", err)
	}

	if count == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *Storage) RemoveRecord(ctx context.Context, userID, recordID int64) error {
	q := `DELETE FROM records
		WHERE id = $1 AND user_id = $2`

	res, err := s.db.ExecContext(ctx, q, recordID, userID)
	if err != nil {
		return fmt.Errorf("can't delete record: %w", err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("can't check edited record: %w", err)
	}

	if count == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *Storage) GetRecords(ctx context.Context, userID int64, from, to time.Time) ([]storage.SleepRecord, error) {
	q := `
		SELECT fell_sleep, woke_up FROM records
		WHERE user_id = $1 AND fell_sleep >= $2 AND fell_sleep < $3
		ORDER BY fell_sleep
	`

	rows, err := s.db.QueryContext(ctx, q, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("can't get records: %w", err)
	}
	defer rows.Close()

	var records []storage.SleepRecord
	for rows.Next() {
		var record storage.SleepRecord
		if err := rows.Scan(&record.FellAsleepAT, &record.WokeUpAt); err != nil {
			return nil, fmt.Errorf("can't get records: %w", err)
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can't get records: %w", err)
	}

	return records, nil
}

func (s *Storage) Init(ctx context.Context) error {
	q := `CREATE TABLE IF NOT EXISTS records (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL,
    fell_sleep TIMESTAMPTZ NOT NULL,
    woke_up TIMESTAMPTZ NOT NULL
	)`

	_, err := s.db.ExecContext(ctx, q)
	if err != nil {
		return fmt.Errorf("can't create table: %w", err)
	}

	return nil
}
