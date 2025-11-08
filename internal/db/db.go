package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Conn *pgxpool.Pool

// Init connects to Postgres
func Init() {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	var err error
	Conn, err = pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

	if err = Conn.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v\n", err)
	}

    log.Println("Connected to Postgres successfully")

    // Ensure users.is_active exists for suspend/activate functionality
    ensureIsActiveColumn()
}

// ensureIsActiveColumn adds users.is_active if missing
func ensureIsActiveColumn() {
    ctx := context.Background()
    var exists bool
    err := Conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'is_active'
        )`).Scan(&exists)
    if err != nil {
        log.Printf("schema check failed: %v", err)
        return
    }
    if exists {
        return
    }
    // Attempt to add the column with default TRUE
    _, err = Conn.Exec(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE`)
    if err != nil {
        log.Printf("failed to add is_active column: %v", err)
        return
    }
    // Backfill any NULLs (in case default didn't apply retroactively)
    _, err = Conn.Exec(ctx, `UPDATE users SET is_active = TRUE WHERE is_active IS NULL`)
    if err != nil {
        log.Printf("failed to backfill is_active: %v", err)
        return
    }
    log.Printf("users.is_active column ensured")
}
