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

type TopupRequest struct {
	Amount int64 `json:"amount" validate:"required,min=100"`
}

type TopupResponse struct {
	TopupID string `json:"topup_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// TopupInit creates a new topup record (pending)
func TopupInit(c echo.Context) error {
	req := new(TopupRequest)
	if err := c.Bind(req); err != nil || req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	userID := c.Get("user_id").(string) // comes from JWT middleware

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

	// mock payment URL (later we’ll integrate Flutterwave/Paystack)
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
// ConfirmTopup handler
// -----------------------------

type ConfirmTopupRequest struct {
	TopupID string `json:"topup_id"`
	Status  string `json:"status"` // must be "success"
}

func ConfirmTopup(c echo.Context) error {
	var req ConfirmTopupRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	// Validate UUID
	topupUUID, err := uuid.Parse(req.TopupID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid topup_id"})
	}

	conn := db.Conn
	ctx := context.Background()

	// Fetch topup
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

	// If already completed
	if status == "completed" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "topup already confirmed"})
	}

	// Only accept "success" → mark as "completed"
	if req.Status == "success" {
		_, err = conn.Exec(ctx, `UPDATE topups SET status = 'completed' WHERE id = $1`, topupUUID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not update topup"})
		}

		_, err = conn.Exec(ctx,
			`UPDATE wallets SET balance = balance + $1 WHERE user_id = $2`,
			amount, userID,
		)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not update wallet"})
		}

		var balance int64
		_ = conn.QueryRow(ctx, `SELECT balance FROM wallets WHERE user_id = $1`, userID).Scan(&balance)

		return c.JSON(http.StatusOK, echo.Map{
			"message": "Topup confirmed and wallet updated",
			"balance": balance,
		})
	}

	return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid status"})
}
