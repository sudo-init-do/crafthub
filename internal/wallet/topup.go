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
