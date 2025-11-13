package marketplace

import (
    "context"
    "database/sql"
    "net/http"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
    "github.com/sudo-init-do/crafthub/internal/alerts"
)

// ReleaseOrder - Admin manually releases escrowed funds to the seller after confirmation.
// This ensures the seller gets paid even if automatic release failed.
func ReleaseOrder(c echo.Context) error {
	adminID, ok := c.Get("user_id").(string)
	if !ok || adminID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	orderID := c.Param("id")
	if orderID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid order id"})
	}

	ctx := context.Background()

	var buyerID, sellerID string
    var amount int64
    var status string

	// Fetch order details
	err := db.Conn.QueryRow(ctx,
		`SELECT buyer_id, seller_id, amount, status 
		 FROM orders 
		 WHERE id = $1`, orderID,
	).Scan(&buyerID, &sellerID, &amount, &status)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch order details"})
	}

    // Only release if order is ready for it
    if status != "in_progress" && status != "delivered" {
        return c.JSON(http.StatusBadRequest, echo.Map{
            "error": "order not in a releasable state",
            "status": status,
        })
    }

	// Begin DB transaction
	tx, err := db.Conn.Begin(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	// Deduct escrow from buyer (if not already released)
	_, err = tx.Exec(ctx,
		`UPDATE wallets 
		 SET escrow = escrow - $1 
		 WHERE user_id = $2 AND escrow >= $1`,
		amount, buyerID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to deduct buyer escrow"})
	}

	// Credit seller’s wallet
	_, err = tx.Exec(ctx,
		`UPDATE wallets 
		 SET balance = balance + $1 
		 WHERE user_id = $2`,
		amount, sellerID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to credit seller wallet"})
	}

	// Update order status → completed
	_, err = tx.Exec(ctx,
		`UPDATE orders 
		 SET status = 'completed', updated_at = NOW() 
		 WHERE id = $1`,
		orderID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update order status"})
	}

    // Log seller credit transaction
    _, err = tx.Exec(ctx,
        `INSERT INTO transactions (user_id, amount, type, status, reference, created_at)
         VALUES ($1, $2, 'credit', 'credited', $3, $4)`,
        sellerID, amount, orderID, time.Now(),
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record seller transaction"})
    }

	// Commit transaction
    if err = tx.Commit(ctx); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to commit transaction"})
    }

    // Admin alert (best-effort)
    _ = alerts.EnqueueAdminAlert(adminID, "info", "Order released: "+orderID)

    // Success response
    return c.JSON(http.StatusOK, echo.Map{
        "message":   "Escrow funds released successfully.",
        "order_id":  orderID,
        "seller_id": sellerID,
    })
}
