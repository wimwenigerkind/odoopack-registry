package database

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

const (
	schemaWaitFirst = time.Second
	schemaWaitMax   = 30 * time.Second
)

// WaitForSchema blocks until SchemaReady holds or ctx ends. The worker does not
// migrate; on a fresh install it can start before the migrate init container of
// the web pods has created the schema, and polling the queue then would only
// log an error per poll.
func WaitForSchema(ctx context.Context, db *gorm.DB) error {
	delay := schemaWaitFirst
	for {
		ready, err := SchemaReady(ctx, db)
		if ready {
			return nil
		}
		if err != nil {
			slog.Warn("check the database schema", "retry_in", delay, "err", err)
		} else {
			slog.Info("waiting for the database schema", "retry_in", delay)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay = nextSchemaWait(delay)
	}
}

func nextSchemaWait(delay time.Duration) time.Duration {
	return min(2*delay, schemaWaitMax)
}
