package wallet

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

// InitWithdrawal handles withdrawal initialization
func InitWithdrawal(c echo.Context) error {
	// Extract user claims from JWT
	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)

	 
	uid, ok := claims["id"].(string)
	if !ok || uid == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid or missing user ID"})
	}

	// Parse request body
	var req struct {
		Amount float64 `json:"amount"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}
	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "amount must be greater than zero"})
	}

	// Generate withdrawal ID
	withdrawalID := uuid.New().String()

	// Insert into withdrawals table
	_, err := db.Conn.Exec(
		c.Request().Context(),
		`INSERT INTO withdrawals (id, user_id, amount, status) VALUES ($1, $2, $3, $4)`,
		withdrawalID, uid, req.Amount, "pending",
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create withdrawal"})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"withdrawal_id": withdrawalID,
		"status":        "pending",
		"message":       "Withdrawal initialized. Awaiting confirmation.",
	})
}

// ConfirmWithdrawal handles withdrawal confirmation (completed/failed)
func ConfirmWithdrawal(c echo.Context) error {
	var req struct {
		WithdrawalID string `json:"withdrawal_id"`
		Status       string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	// Only allow valid statuses
	if req.Status != "completed" && req.Status != "failed" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid status"})
	}

	// Update withdrawal in DB
	_, err := db.Conn.Exec(
		c.Request().Context(),
		`UPDATE withdrawals SET status = $1 WHERE id = $2`,
		req.Status, req.WithdrawalID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not update withdrawal"})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"withdrawal_id": req.WithdrawalID,
		"status":        req.Status,
		"message":       "Withdrawal status updated successfully.",
	})
}
