package wallet

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

// InitWithdrawal handles immediate user withdrawals (no admin approval)
func InitWithdrawal(c echo.Context) error {
	// Get user ID from JWT context
	uid, ok := c.Get("user_id").(string)
	if !ok || uid == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": "unauthorized or invalid user",
		})
	}

	// Parse request
	var req struct {
		Amount int64 `json:"amount"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid request body",
		})
	}
	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "amount must be greater than zero",
		})
	}

	ctx := context.Background()

	// Check wallet balance
	var balance int64
	err := db.Conn.QueryRow(ctx, `SELECT balance FROM wallets WHERE user_id = $1`, uid).Scan(&balance)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "could not fetch wallet balance",
		})
	}

	// Insufficient balance
	if req.Amount > balance {
		// Log failed transaction
		_, _ = db.Conn.Exec(ctx,
			`INSERT INTO transactions (id, user_id, type, amount, status, created_at)
			 VALUES ($1, $2, 'withdrawal', $3, 'failed', $4)`,
			uuid.New().String(), uid, req.Amount, time.Now(),
		)
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "insufficient balance",
		})
	}

	// Begin database transaction
	tx, err := db.Conn.Begin(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "could not start transaction",
		})
	}
	defer tx.Rollback(ctx)

	// Deduct wallet balance
	_, err = tx.Exec(ctx, `UPDATE wallets SET balance = balance - $1 WHERE user_id = $2`, req.Amount, uid)
	if err != nil {
		// Log failed transaction
		_, _ = tx.Exec(ctx,
			`INSERT INTO transactions (id, user_id, type, amount, status, created_at)
			 VALUES ($1, $2, 'withdrawal', $3, 'failed', $4)`,
			uuid.New().String(), uid, req.Amount, time.Now(),
		)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "could not update wallet balance",
		})
	}

	// Log successful transaction
	withdrawalID := uuid.New().String()
	_, err = tx.Exec(ctx,
		`INSERT INTO transactions (id, user_id, type, amount, status, created_at)
		 VALUES ($1, $2, 'withdrawal', $3, 'completed', $4)`,
		withdrawalID, uid, req.Amount, time.Now(),
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "could not record transaction",
		})
	}

	// Commit the transaction
	if err = tx.Commit(ctx); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "could not finalize withdrawal",
		})
	}

	// Return response
	return c.JSON(http.StatusOK, echo.Map{
		"withdrawal_id": withdrawalID,
		"amount":        req.Amount,
		"status":        "completed",
		"message":       "Withdrawal successful and balance updated",
	})
}
