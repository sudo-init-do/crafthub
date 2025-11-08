package wallet

import (
    "context"
    "net/http"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

// AdminTransactionResponse is used for admin API responses
type AdminTransactionResponse struct {
    ID        string  `json:"id"`
    UserID    string  `json:"user_id"`
    Type      string  `json:"type"`
    Amount    float64 `json:"amount"`
    Status    string  `json:"status"`
    CreatedAt time.Time `json:"created_at"`
}

// AdminGetAllTransactions returns all transactions for admin monitoring
func AdminGetAllTransactions(c echo.Context) error {
	rows, err := db.Conn.Query(
		context.Background(),
		`SELECT id, user_id, type, amount, status, created_at
		 FROM transactions
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch transactions"})
	}
	defer rows.Close()

	var txs []AdminTransactionResponse
	for rows.Next() {
		var t AdminTransactionResponse
		if err := rows.Scan(&t.ID, &t.UserID, &t.Type, &t.Amount, &t.Status, &t.CreatedAt); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to read transaction record"})
		}
		txs = append(txs, t)
	}

	return c.JSON(http.StatusOK, echo.Map{"transactions": txs})
}

// AdminGetUserTransactions returns all transactions for a specific user (admin view)
func AdminGetUserTransactions(c echo.Context) error {
	userID := c.Param("id")
	if userID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "user ID is required"})
	}

	rows, err := db.Conn.Query(
		context.Background(),
		`SELECT id, user_id, type, amount, status, created_at
		 FROM transactions
		 WHERE user_id = $1
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch user transactions"})
	}
	defer rows.Close()

	var txs []AdminTransactionResponse
	for rows.Next() {
		var t AdminTransactionResponse
		if err := rows.Scan(&t.ID, &t.UserID, &t.Type, &t.Amount, &t.Status, &t.CreatedAt); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to read transaction record"})
		}
		txs = append(txs, t)
	}

	return c.JSON(http.StatusOK, echo.Map{"transactions": txs})
}
