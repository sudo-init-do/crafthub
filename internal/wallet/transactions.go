package wallet

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

// Transaction model for responses
type Transaction struct {
	ID        string    `json:"id"`
	Amount    int       `json:"amount"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// TransactionsHandler returns all topups for the authenticated user
func TransactionsHandler(c echo.Context) error {
	// Safely extract user_id from context
	uid, ok := c.Get("user_id").(string)
	if !ok || uid == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "unauthorized or invalid user",
		})
	}

	// Query topups for this user (with context)
	rows, err := db.Conn.Query(
		context.Background(),
		`SELECT id, amount, status, created_at
		 FROM topups
		 WHERE user_id = $1
		 ORDER BY created_at DESC`,
		uid,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch transactions"})
	}
	defer rows.Close()

	var txs []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.Amount, &t.Status, &t.CreatedAt); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "scan error"})
		}
		txs = append(txs, t)
	}

	return c.JSON(http.StatusOK, txs)
}
