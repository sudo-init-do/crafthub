package wallet

import (
    "context"
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

// InitWithdrawal initializes a withdrawal request (two-step flow)
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
    if req.Amount < 100 {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "amount must be at least 100"})
    }
    if req.Amount > 10_000_000 {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "amount exceeds maximum limit"})
    }

    ctx := context.Background()
    // Create a pending withdrawal entry; funds will be deducted on confirmation
    withdrawalID := uuid.New().String()
    _, err := db.Conn.Exec(ctx,
        `INSERT INTO withdrawals (id, user_id, amount, status, created_at)
         VALUES ($1, $2, $3, 'pending', $4)`,
        withdrawalID, uid, req.Amount, time.Now(),
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create withdrawal"})
    }

    return c.JSON(http.StatusOK, echo.Map{
        "withdrawal_id": withdrawalID,
        "amount":        req.Amount,
        "status":        "pending",
        "message":       "Withdrawal initialized; confirm to complete",
    })
}
