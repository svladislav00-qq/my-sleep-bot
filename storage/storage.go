package storage

import (
	"context"
	"errors"
	"time"
)

type Storage interface {
	SaveRecord(ctx context.Context, record *SleepRecord) error
	EditRecord(ctx context.Context, record *SleepRecord) error
	RemoveRecord(ctx context.Context, userID, recordID int64) error
	GetRecords(ctx context.Context, userID int64, from, to time.Time) ([]SleepRecord, error)
}

var ErrNoSavedTime = errors.New("no time saved")

type SleepRecord struct {
	ID           int64
	UserID       int64
	FellAsleepAT time.Time
	WokeUpAt     time.Time
}
