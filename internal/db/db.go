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

    // Ensure services columns used for discovery and moderation exist
    ensureServicesColumns()

    // Ensure wallets columns used by orders exist
    ensureWalletColumns()

    // Ensure orders schema and status constraint match handlers
    ensureOrdersSchema()

    // Ensure notifications table exists for in-app alerts
    ensureNotificationsTable()

    // Ensure disputes table exists for dispute management
    ensureDisputesTable()
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

// ensureServicesColumns adds services.status and services.delivery_time_days if missing
func ensureServicesColumns() {
    ctx := context.Background()
    // status column
    var statusExists bool
    _ = Conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'services' AND column_name = 'status'
        )`).Scan(&statusExists)
    if !statusExists {
        if _, err := Conn.Exec(ctx, `ALTER TABLE services ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'active'`); err != nil {
            log.Printf("failed to add services.status: %v", err)
        }
    }

    // delivery_time_days column
    var deliveryExists bool
    _ = Conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'services' AND column_name = 'delivery_time_days'
        )`).Scan(&deliveryExists)
    if !deliveryExists {
        if _, err := Conn.Exec(ctx, `ALTER TABLE services ADD COLUMN IF NOT EXISTS delivery_time_days INTEGER`); err != nil {
            log.Printf("failed to add services.delivery_time_days: %v", err)
        }
    }
}

// ensureWalletColumns adds wallets.locked_amount and wallets.escrow if missing
func ensureWalletColumns() {
    ctx := context.Background()
    // locked_amount
    var lockedExists bool
    _ = Conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'wallets' AND column_name = 'locked_amount'
        )`).Scan(&lockedExists)
    if !lockedExists {
        if _, err := Conn.Exec(ctx, `ALTER TABLE wallets ADD COLUMN IF NOT EXISTS locked_amount BIGINT DEFAULT 0`); err != nil {
            log.Printf("failed to add wallets.locked_amount: %v", err)
        } else {
            // Backfill any NULLs
            _, _ = Conn.Exec(ctx, `UPDATE wallets SET locked_amount = 0 WHERE locked_amount IS NULL`)
        }
    }

    // escrow
    var escrowExists bool
    _ = Conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'wallets' AND column_name = 'escrow'
        )`).Scan(&escrowExists)
    if !escrowExists {
        if _, err := Conn.Exec(ctx, `ALTER TABLE wallets ADD COLUMN IF NOT EXISTS escrow BIGINT DEFAULT 0`); err != nil {
            log.Printf("failed to add wallets.escrow: %v", err)
        } else {
            _, _ = Conn.Exec(ctx, `UPDATE wallets SET escrow = 0 WHERE escrow IS NULL`)
        }
    }
}

// ensureOrdersSchema ensures orders table has updated_at column and a compatible status constraint
func ensureOrdersSchema() {
    ctx := context.Background()

    // Ensure updated_at exists
    var updatedExists bool
    _ = Conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'orders' AND column_name = 'updated_at'
        )`).Scan(&updatedExists)
    if !updatedExists {
        if _, err := Conn.Exec(ctx, `ALTER TABLE orders ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT NOW()`); err != nil {
            log.Printf("failed to add orders.updated_at: %v", err)
        }
    }

    // Relax or update status CHECK constraint to include runtime statuses used by handlers
    // Attempt to drop the existing auto-named check constraint if present
    _, _ = Conn.Exec(ctx, `ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check`)

    // Add a new named CHECK constraint with a superset of statuses used across code and older migrations
    _, err := Conn.Exec(ctx, `
        ALTER TABLE orders
        ADD CONSTRAINT orders_status_check
        CHECK (status IN (
            'pending', 'accepted', 'rejected', 'completed', 'cancelled', 'confirmed',
            'pending_acceptance', 'in_progress', 'delivered', 'declined', 'canceled'
        ))`)
    if err != nil {
        log.Printf("failed to update orders status constraint: %v", err)
    }
}

// ensureNotificationsTable creates notifications table if it doesn't exist
func ensureNotificationsTable() {
    ctx := context.Background()
    var exists bool
    _ = Conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM information_schema.tables
            WHERE table_schema = 'public' AND table_name = 'notifications'
        )`).Scan(&exists)
    if exists {
        return
    }
    _, err := Conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS notifications (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            type TEXT NOT NULL,
            title TEXT NOT NULL,
            body TEXT,
            reference UUID NULL,
            metadata JSONB NULL,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
            read_at TIMESTAMP WITH TIME ZONE NULL
        );
        CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at);
        CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id) WHERE read_at IS NULL;
    `)
    if err != nil {
        log.Printf("failed to create notifications table: %v", err)
    }
}

// ensureDisputesTable creates disputes table if not present
func ensureDisputesTable() {
    ctx := context.Background()
    var exists bool
    _ = Conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM information_schema.tables
            WHERE table_schema = 'public' AND table_name = 'disputes'
        )`).Scan(&exists)
    if exists { return }
    _, err := Conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS disputes (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
            filer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            reason TEXT NOT NULL,
            status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','resolved')),
            resolution TEXT NULL CHECK (resolution IN ('refund','release','none')),
            notes TEXT NULL,
            resolved_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
            resolved_at TIMESTAMP WITH TIME ZONE NULL
        );
        CREATE INDEX IF NOT EXISTS idx_disputes_order ON disputes(order_id);
        CREATE INDEX IF NOT EXISTS idx_disputes_filer ON disputes(filer_id);
        CREATE INDEX IF NOT EXISTS idx_disputes_status ON disputes(status);
    `)
    if err != nil {
        log.Printf("failed to create disputes table: %v", err)
    }
}
