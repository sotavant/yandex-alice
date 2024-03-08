package store

import (
	"context"
	"time"
)

type Store interface {
	FindRecipient(ctx context.Context, username string) (userId string, err error)
	ListMessages(ctx context.Context, userID string) ([]Message, error)
	GetMessage(ctx context.Context, id int64) (*Message, error)
	SaveMessage(ctx context.Context, userID string, msg Message) error
}

type Message struct {
	ID      int64
	Sender  string
	Time    time.Time
	Payload string
}
