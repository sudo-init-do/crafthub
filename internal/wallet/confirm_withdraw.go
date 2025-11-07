package wallet

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

type ConfirmWithdrawalRequest struct {
	WithdrawalID string `json:"withdrawal_id"`
}

func ConfirmWithdrawal(c echo.Context) error {
	uid, ok := c.Get("user_id").(string)
	if !ok || uid == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	var req ConfirmWithdrawalRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	ctx := context.Background()

	tx, err := db.Conn.Begin(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not start transaction"})
	}
	defer tx.Rollback(ctx)

	var userID string
	var amount int64
	var status string
	err = tx.QueryRow(ctx,
		`SELECT user_id, amount, status FROM withdrawals WHERE id=$1 AND user_id=$2 FOR UPDATE`,
		req.WithdrawalID, uid,
	).Scan(&userID, &amount, &status)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "withdrawal not found"})
	}

	if status == "completed" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "withdrawal already completed"})
	}
	if status == "rejected" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "withdrawal rejected"})
	}

	// Check wallet balance
	var balance int64
	if err = tx.QueryRow(ctx, `SELECT balance FROM wallets WHERE user_id = $1 FOR UPDATE`, userID).Scan(&balance); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch wallet balance"})
	}
	if amount > balance {
		// Mark withdrawal failed
		_, _ = tx.Exec(ctx, `UPDATE withdrawals SET status='failed' WHERE id=$1`, req.WithdrawalID)
		// Log failed transaction
		_, _ = tx.Exec(ctx,
			`INSERT INTO transactions (id, user_id, type, amount, status, created_at)
             VALUES ($1, $2, 'withdrawal', $3, 'failed', $4)`,
			uuid.New().String(), userID, amount, time.Now(),
		)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "insufficient balance"})
	}

	// Deduct and complete
	if _, err = tx.Exec(ctx, `UPDATE wallets SET balance = balance - $1 WHERE user_id = $2`, amount, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not update wallet"})
	}
	if _, err = tx.Exec(ctx, `UPDATE withdrawals SET status='completed' WHERE id=$1`, req.WithdrawalID); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not update withdrawal"})
	}
	_, _ = tx.Exec(ctx,
		`INSERT INTO transactions (id, user_id, type, amount, status, created_at)
         VALUES ($1, $2, 'withdrawal', $3, 'completed', $4)`,
		uuid.New().String(), userID, amount, time.Now(),
	)

	if err = tx.Commit(ctx); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not finalize withdrawal"})
	}

	var newBalance int64
	_ = db.Conn.QueryRow(ctx, `SELECT balance FROM wallets WHERE user_id = $1`, userID).Scan(&newBalance)
	return c.JSON(http.StatusOK, echo.Map{"message": "withdrawal completed", "balance": newBalance})
}
