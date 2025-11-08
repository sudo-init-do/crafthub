package main

import (
    "context"
    "flag"
    "fmt"
    "log"

    "github.com/sudo-init-do/crafthub/internal/db"
)

// promote_creator sets a user's role to 'creator' by email.
// Usage:
//   go run cmd/adminutil/promote_creator/main.go -email user@example.com
func main() {
    email := flag.String("email", "", "Email of the user to promote to creator")
    flag.Parse()

    if *email == "" {
        log.Fatalf("usage: go run cmd/adminutil/promote_creator/main.go -email user@example.com")
    }

    // Initialize DB from environment variables
    db.Init()

    // Ensure constraints/columns are in place (idempotent)
    _, err := db.Conn.Exec(context.Background(), `
        ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
        ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('fan','creator','admin'));
        ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE;
    `)
    if err != nil {
        log.Fatalf("failed to update users table constraints/columns: %v", err)
    }

    // Promote the user to creator
    ct, err := db.Conn.Exec(context.Background(), `UPDATE users SET role = 'creator' WHERE email = $1`, *email)
    if err != nil {
        log.Fatalf("failed to promote user to creator: %v", err)
    }

    if ct.RowsAffected() == 0 {
        log.Fatalf("no user found with email: %s", *email)
    }

    fmt.Printf("User %s promoted to creator.\n", *email)
}

