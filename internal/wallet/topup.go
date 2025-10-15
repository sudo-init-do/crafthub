package wallet

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

// -----------------------------
// Request / Response Models
// -----------------------------
type TopupRequest struct {
	Amount int64 `json:"amount" validate:"required,min=100"`
}

type TopupResponse struct {
	TopupID string `json:"topup_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ConfirmTopupRequest struct {
	TopupID string `json:"topup_id"`
	Status  string `json:"status"` // must be "success"
}

// -----------------------------
// TopupInit - Create Pending Record
// -----------------------------
func TopupInit(c echo.Context) error {
	req := new(TopupRequest)
	if err := c.Bind(req); err != nil || req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	userID := c.Get("user_id").(string)
	conn := db.Conn
	ctx := context.Background()

	topupID := uuid.New().String()
	createdAt := time.Now()

	_, err := conn.Exec(ctx,
		`INSERT INTO topups (id, user_id, amount, status, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		topupID, userID, req.Amount, "pending", createdAt,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create topup"})
	}

	paymentURL := os.Getenv("MOCK_PAYMENT_URL")
	if paymentURL == "" {
		paymentURL = "https://pay.crafthub.dev/mock/" + topupID
	}

	return c.JSON(http.StatusOK, TopupResponse{
		TopupID: topupID,
		Status:  "pending",
		Message: "Topup initialized. Complete payment at " + paymentURL,
	})
}

// -----------------------------
// ConfirmTopup - Confirm Payment + Credit Wallet
// -----------------------------
func ConfirmTopup(c echo.Context) error {
	var req ConfirmTopupRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	topupUUID, err := uuid.Parse(req.TopupID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid topup_id"})
	}

	conn := db.Conn
	ctx := context.Background()

	var userID string
	var amount int64
	var status string
	err = conn.QueryRow(ctx,
		`SELECT user_id, amount, status 
		 FROM topups 
		 WHERE id = $1`, topupUUID,
	).Scan(&userID, &amount, &status)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "topup not found"})
	}

	if status == "completed" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "topup already confirmed"})
	}

	if req.Status != "success" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid status"})
	}

	// Mark topup as completed
	_, err = conn.Exec(ctx, `UPDATE topups SET status = 'completed' WHERE id = $1`, topupUUID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not update topup"})
	}

	// Update wallet balance
	_, err = conn.Exec(ctx,
		`UPDATE wallets SET balance = balance + $1 WHERE user_id = $2`,
		amount, userID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not update wallet"})
	}

	// Log transaction
	_, _ = conn.Exec(ctx,
		`INSERT INTO transactions (id, user_id, type, amount, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New().String(), userID, "deposit", amount, "completed", time.Now(),
	)

	// Return new balance
	var balance int64
	_ = conn.QueryRow(ctx, `SELECT balance FROM wallets WHERE user_id = $1`, userID).Scan(&balance)

	return c.JSON(http.StatusOK, echo.Map{
		"message": "Topup confirmed and wallet updated",
		"balance": balance,
	})
}

// -----------------------------
// ListPendingTopups - Admin Only
// -----------------------------
func ListPendingTopups(c echo.Context) error {
	ctx := context.Background()

	rows, err := db.Conn.Query(ctx,
		`SELECT id, user_id, amount, status, created_at
		 FROM topups
		 WHERE status = 'pending'
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch pending topups"})
	}
	defer rows.Close()

	var topups []map[string]interface{}
	for rows.Next() {
		var id, userID, status string
		var amount int64
		var createdAt time.Time

		if err := rows.Scan(&id, &userID, &amount, &status, &createdAt); err == nil {
			topups = append(topups, map[string]interface{}{
				"id":         id,
				"user_id":    userID,
				"amount":     amount,
				"status":     status,
				"created_at": createdAt,
			})
		}
	}

	return c.JSON(http.StatusOK, echo.Map{
		"pending_topups": topups,
	})
}
