package marketplace

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/alerts"
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
	var amount int64
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

	if status != "pending_acceptance" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "order not awaiting acceptance"})
	}

	// Fetch buyer wallet balance
	var balance int64
	var locked int64
	err = db.Conn.QueryRow(ctx,
		`SELECT balance, locked_amount FROM wallets WHERE user_id = $1`, buyerID,
	).Scan(&balance, &locked)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "buyer wallet not found"})
	}
	if locked < amount {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "held funds unavailable"})
	}

	// Begin transaction
	tx, err := db.Conn.Begin(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction start failed"})
	}
	defer tx.Rollback(ctx)

	// Convert hold to debit and move to escrow
	_, err = tx.Exec(ctx,
		`UPDATE wallets SET locked_amount = locked_amount - $1, balance = balance - $1, escrow = escrow + $1 WHERE user_id = $2`,
		amount, buyerID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to convert hold to escrow"})
	}

	// Update order status to 'in_progress'
	_, err = tx.Exec(ctx,
		`UPDATE orders SET status = 'in_progress', updated_at = NOW() WHERE id = $1`,
		orderID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update order status"})
	}

	// Update the pending hold transaction to 'debited'
	_, err = tx.Exec(ctx,
		`UPDATE transactions SET status = 'debited' WHERE user_id = $1 AND reference = $2 AND status = 'pending_hold'`,
		buyerID, orderID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update transaction status"})
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "commit failed"})
	}

	// Lookup buyer email for confirmation notification
	var buyerEmail string
	_ = db.Conn.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, buyerID).Scan(&buyerEmail)
	if buyerEmail != "" {
		_ = alerts.EnqueueBookingConfirmation(orderID, buyerID, sellerID, buyerEmail, float64(amount))
	}

	return c.JSON(http.StatusOK, echo.Map{
		"message": "Order accepted; funds debited and work in progress",
	})
}
