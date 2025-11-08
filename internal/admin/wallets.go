package admin

import (
    "context"
    "net/http"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

type AdminWallet struct {
    UserID    string    `json:"user_id"`
    Balance   int64     `json:"balance"`
    Escrow    int64     `json:"escrow"`
    CreatedAt time.Time `json:"created_at"`
}

// GET /admin/wallets
func ListWallets(c echo.Context) error {
    rows, err := db.Conn.Query(context.Background(),
        `SELECT user_id, balance, escrow, created_at FROM wallets ORDER BY created_at DESC`,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch wallets"})
    }
    defer rows.Close()

    var wallets []AdminWallet
    for rows.Next() {
        var w AdminWallet
        if err := rows.Scan(&w.UserID, &w.Balance, &w.Escrow, &w.CreatedAt); err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to read wallet record"})
        }
        wallets = append(wallets, w)
    }
    return c.JSON(http.StatusOK, echo.Map{"wallets": wallets})
}
