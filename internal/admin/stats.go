package admin

import (
    "context"
    "net/http"

    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

// GET /admin/stats
func Stats(c echo.Context) error {
    ctx := context.Background()

    var users, services, orders, wallets, transactions int

    _ = db.Conn.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&users)
    _ = db.Conn.QueryRow(ctx, `SELECT COUNT(*) FROM services`).Scan(&services)
    _ = db.Conn.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&orders)
    _ = db.Conn.QueryRow(ctx, `SELECT COUNT(*) FROM wallets`).Scan(&wallets)
    _ = db.Conn.QueryRow(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&transactions)

    return c.JSON(http.StatusOK, echo.Map{
        "users":        users,
        "services":     services,
        "orders":       orders,
        "wallets":      wallets,
        "transactions": transactions,
    })
}

