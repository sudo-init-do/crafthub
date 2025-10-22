package marketplace

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

// ConfirmOrder - Seller confirms a pending order and deducts buyer funds (escrow)
func ConfirmOrder(c echo.Context) error {
	sellerID, ok := c.Get("user_id").(string)
	if !ok || sellerID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	orderID := c.Param("id")
	if orderID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid order id"})
	}

	ctx := context.Background()

	// Fetch order details for the seller
	var buyerID string
	var amount float64
	var status string
	err := db.Conn.QueryRow(ctx,
		`SELECT buyer_id, amount, status FROM orders WHERE id = $1 AND seller_id = $2`,
		orderID, sellerID,
	).Scan(&buyerID, &amount, &status)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found or not yours"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch order"})
	}

	if status != "pending" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "order not pending"})
	}

	// Fetch buyer wallet balance
	var balance float64
	err = db.Conn.QueryRow(ctx,
		`SELECT balance FROM wallets WHERE user_id = $1`, buyerID,
	).Scan(&balance)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "buyer wallet not found"})
	}

	if balance < amount {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "buyer has insufficient balance"})
	}

	// Begin transaction
	tx, err := db.Conn.Begin(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction start failed"})
	}
	defer tx.Rollback(ctx)

	// Deduct buyer funds (hold in escrow)
	_, err = tx.Exec(ctx,
		`UPDATE wallets SET balance = balance - $1 WHERE user_id = $2`,
		amount, buyerID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to deduct buyer funds"})
	}

	// Update order status to 'confirmed'
	_, err = tx.Exec(ctx,
		`UPDATE orders SET status = 'confirmed', updated_at = NOW() WHERE id = $1`,
		orderID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update order status"})
	}

	// Record transaction for buyer (debit)
	_, err = tx.Exec(ctx,
		`INSERT INTO transactions (user_id, amount, type, status, created_at)
		 VALUES ($1, $2, 'debit', 'completed', $3)`,
		buyerID, amount, time.Now(),
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record transaction"})
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "commit failed"})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"message": "Order confirmed successfully. Buyer funds held in escrow.",
	})
}
