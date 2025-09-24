package wallet

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

// Balance returns the authenticated user's wallet balance
func Balance(c echo.Context) error {
	// Extract user_id set by JWT middleware (or parsed manually)
	userID := c.Get("user_id")
	if userID == nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	var balance int64
	err := db.Conn.QueryRow(context.Background(),
		`SELECT balance FROM wallets WHERE user_id=$1`, userID).
		Scan(&balance)

	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "wallet not found"})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"user_id": userID,
		"balance": balance,
	})
}
