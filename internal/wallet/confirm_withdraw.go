package wallet

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

type ConfirmWithdrawalRequest struct {
	WithdrawalID string `json:"withdrawal_id"`
	Reference    string `json:"reference"`
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

	// Verify the withdrawal belongs to the user
	var exists bool
	err := db.Conn.QueryRow(
		context.Background(),
		`SELECT EXISTS(SELECT 1 FROM withdrawals WHERE id=$1 AND user_id=$2)`,
		req.WithdrawalID, uid,
	).Scan(&exists)
	if err != nil || !exists {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "withdrawal not found"})
	}

	// Update status to confirmed
	_, err = db.Conn.Exec(
		context.Background(),
		`UPDATE withdrawals SET status='confirmed', reference=$1 WHERE id=$2`,
		req.Reference, req.WithdrawalID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not confirm withdrawal"})
	}

	return c.JSON(http.StatusOK, echo.Map{"message": "withdrawal confirmed"})
}
