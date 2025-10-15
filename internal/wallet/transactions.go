package wallet

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

// Transaction model for responses
type Transaction struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Amount    float64   `json:"amount"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateTransaction inserts a new transaction record
func CreateTransaction(ctx context.Context, userID string, txType string, amount float64, status string) error {
	_, err := db.Conn.Exec(
		ctx,
		`INSERT INTO transactions (id, user_id, type, amount, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New().String(), userID, txType, amount, status, time.Now(),
	)
	return err
}

// GetUserTransactions returns all transactions for a given user
func GetUserTransactions(c echo.Context) error {
	uid, ok := c.Get("user_id").(string)
	if !ok || uid == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized or invalid user"})
	}

	rows, err := db.Conn.Query(
		context.Background(),
		`SELECT id, user_id, type, amount, status, created_at
		 FROM transactions
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
		if err := rows.Scan(&t.ID, &t.UserID, &t.Type, &t.Amount, &t.Status, &t.CreatedAt); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to read transaction record"})
		}
		txs = append(txs, t)
	}

	return c.JSON(http.StatusOK, echo.Map{"transactions": txs})
}

// GetAllTransactions returns transactions for all users (for admin)
func GetAllTransactions(c echo.Context) error {
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

	var txs []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.UserID, &t.Type, &t.Amount, &t.Status, &t.CreatedAt); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to read transaction record"})
		}
		txs = append(txs, t)
	}

	return c.JSON(http.StatusOK, echo.Map{"transactions": txs})
}
