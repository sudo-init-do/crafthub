package wallet

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/sudo-init-do/crafthub/internal/db"
)

// WithdrawActionRequest represents the admin action request
type WithdrawActionRequest struct {
	AdminID uuid.UUID `json:"admin_id"`
}

// ListPendingWithdrawals returns all withdrawals with status "pending"
func ListPendingWithdrawals(c echo.Context) error {
	rows, err := db.Conn.Query(c.Request().Context(),
		`SELECT id, user_id, amount, status, created_at 
		 FROM withdrawals 
		 WHERE status = 'pending'`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "failed to fetch withdrawals",
		})
	}
	defer rows.Close()

	var withdrawals []map[string]interface{}
	for rows.Next() {
		var id, userID uuid.UUID
		var amount int64
		var status string
		var createdAt time.Time

		if err := rows.Scan(&id, &userID, &amount, &status, &createdAt); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{
				"error": "failed to scan withdrawals",
			})
		}

		withdrawals = append(withdrawals, map[string]interface{}{
			"id":         id,
			"user_id":    userID,
			"amount":     amount,
			"status":     status,
			"created_at": createdAt,
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"pending_withdrawals": withdrawals,
	})
}

// ApproveWithdrawal marks a withdrawal as approved
func ApproveWithdrawal(c echo.Context) error {
	id := c.Param("id")
	var req WithdrawActionRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid request",
		})
	}

	_, err := db.Conn.Exec(c.Request().Context(),
		`UPDATE withdrawals 
		 SET status = 'approved', approved_by = $1, approved_at = $2 
		 WHERE id = $3 AND status = 'pending'`,
		req.AdminID, time.Now(), id,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "failed to approve withdrawal",
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"message":      "withdrawal approved",
		"withdrawal_id": id,
		"status":       "approved",
	})
}

// RejectWithdrawal marks a withdrawal as rejected
func RejectWithdrawal(c echo.Context) error {
	id := c.Param("id")
	var req WithdrawActionRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid request",
		})
	}

	_, err := db.Conn.Exec(c.Request().Context(),
		`UPDATE withdrawals 
		 SET status = 'rejected', approved_by = $1, approved_at = $2 
		 WHERE id = $3 AND status = 'pending'`,
		req.AdminID, time.Now(), id,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "failed to reject withdrawal",
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"message":      "withdrawal rejected",
		"withdrawal_id": id,
		"status":       "rejected",
	})
}
