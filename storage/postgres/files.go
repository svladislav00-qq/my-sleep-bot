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

func (s Storage) SaveRecord(ctx context.Context, record *storage.SleepRecord) (err error) {

}

func (s Storage) EditRecord(ctx context.Context, record *storage.SleepRecord) (err error) {

}

func (s Storage) RemoveRecord(ctx context.Context, userID, recordID int64) (err error) {

}

func (s Storage) GetRecords(ctx context.Context, userID int64, from, to time.Time) ([]storage.SleepRecord, error) {

}
