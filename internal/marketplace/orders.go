package marketplace

import (
    "context"
    "database/sql"
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
    "github.com/sudo-init-do/crafthub/internal/alerts"
)

// =========================
// CreateOrder - Buyer places order
// =========================
func CreateOrder(c echo.Context) error {
	buyerID, ok := c.Get("user_id").(string)
	if !ok || buyerID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	var req struct {
		ServiceID string `json:"service_id"`
	}
	if err := c.Bind(&req); err != nil || req.ServiceID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid service_id"})
	}

    var sellerID string
    var price int64
    err := db.Conn.QueryRow(context.Background(),
        `SELECT user_id, price FROM services WHERE id = $1`,
        req.ServiceID,
    ).Scan(&sellerID, &price)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch service"})
	}

	if sellerID == buyerID {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "you cannot order your own service"})
	}

    var balance int64
    var locked int64
    err = db.Conn.QueryRow(context.Background(),
        `SELECT balance, locked_amount FROM wallets WHERE user_id = $1`,
        buyerID,
    ).Scan(&balance, &locked)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "wallet not found"})
    }
    // Available balance considers locked funds
    available := balance - locked
    if available < price {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "insufficient available balance"})
    }

    // Begin transaction: reserve funds and create order + transaction
    tx, err := db.Conn.Begin(context.Background())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction start failed"})
    }
    defer tx.Rollback(context.Background())

    orderID := uuid.New().String()

    // Reserve funds by increasing locked_amount (do not deduct balance yet)
    _, err = tx.Exec(context.Background(),
        `UPDATE wallets SET locked_amount = locked_amount + $1 WHERE user_id = $2`,
        price, buyerID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to reserve funds"})
    }

    // Create order in pending_acceptance
    _, err = tx.Exec(context.Background(),
        `INSERT INTO orders (id, service_id, buyer_id, seller_id, amount, status, created_at)
         VALUES ($1, $2, $3, $4, $5, 'pending_acceptance', $6)`,
        orderID, req.ServiceID, buyerID, sellerID, price, time.Now(),
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to create order"})
    }

    // Log a pending hold transaction tied to this order
    _, err = tx.Exec(context.Background(),
        `INSERT INTO transactions (user_id, amount, type, status, reference, created_at)
         VALUES ($1, $2, 'debit', 'pending_hold', $3, $4)`,
        buyerID, price, orderID, time.Now(),
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record hold"})
    }

    if err = tx.Commit(context.Background()); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "commit failed"})
    }

    return c.JSON(http.StatusCreated, echo.Map{
        "order_id": orderID,
        "message":  "Order created. Funds reserved pending seller acceptance.",
    })
}

// =========================
// AcceptOrder - Seller accepts order
// =========================
func AcceptOrder(c echo.Context) error {
    sellerID, ok := c.Get("user_id").(string)
    if !ok || sellerID == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
    }

    orderID := c.Param("id")
    if orderID == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id in URL"})
    }
    // Fetch order details
    var buyerID string
    var amount int64
    var status string
    err := db.Conn.QueryRow(context.Background(),
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

    // Begin transaction: convert hold to debit and move to escrow; set status in_progress
    tx, err := db.Conn.Begin(context.Background())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction start failed"})
    }
    defer tx.Rollback(context.Background())

    // Ensure buyer has the held amount
    var balance int64
    var locked int64
    err = tx.QueryRow(context.Background(), `SELECT balance, locked_amount FROM wallets WHERE user_id = $1 FOR UPDATE`, buyerID).Scan(&balance, &locked)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "buyer wallet not found"})
    }
    if locked < amount {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "held funds unavailable"})
    }

    // Convert hold to debit: decrease locked_amount, decrease balance, increase escrow
    _, err = tx.Exec(context.Background(),
        `UPDATE wallets SET locked_amount = locked_amount - $1, balance = balance - $1, escrow = escrow + $1 WHERE user_id = $2`,
        amount, buyerID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to convert hold to escrow"})
    }

    // Update order status to in_progress
    _, err = tx.Exec(context.Background(),
        `UPDATE orders SET status = 'in_progress', updated_at = NOW() WHERE id = $1`,
        orderID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update order status"})
    }

    // Update the pending hold transaction to 'debited'
    _, err = tx.Exec(context.Background(),
        `UPDATE transactions SET status = 'debited' WHERE user_id = $1 AND reference = $2 AND status = 'pending_hold'`,
        buyerID, orderID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update transaction status"})
    }

    if err = tx.Commit(context.Background()); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "commit failed"})
    }

    return c.JSON(http.StatusOK, echo.Map{"message": "Order accepted; funds debited and work in progress"})
}

// =========================
// RejectOrder - Seller rejects order (refunds buyer)
// =========================
func RejectOrder(c echo.Context) error {
    sellerID, ok := c.Get("user_id").(string)
    if !ok || sellerID == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
    }

	orderID := c.Param("id")
	if orderID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id in URL"})
	}

    // Handle rejection for both 'pending' and 'confirmed' (refund escrow if needed)
    var buyerID string
    var amount int64
    var status string
    err := db.Conn.QueryRow(context.Background(),
        `SELECT buyer_id, amount, status FROM orders WHERE id = $1 AND seller_id = $2`,
        orderID, sellerID).Scan(&buyerID, &amount, &status)
    if err != nil {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "order not found or already handled"})
    }

    tx, err := db.Conn.Begin(context.Background())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction start failed"})
    }
    defer tx.Rollback(context.Background())

    if status == "in_progress" || status == "delivered" {
        // Refund escrow to buyer
        _, err = tx.Exec(context.Background(),
            `UPDATE wallets SET escrow = escrow - $1, balance = balance + $1 WHERE user_id = $2 AND escrow >= $1`,
            amount, buyerID,
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to refund escrow"})
        }
        // Log refund transaction
        _, err = tx.Exec(context.Background(),
            `INSERT INTO transactions (user_id, amount, type, status, reference, created_at)
             VALUES ($1, $2, 'credit', 'refunded', $3, $4)`,
            buyerID, amount, orderID, time.Now(),
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record refund"})
        }
    } else if status == "pending_acceptance" {
        // Release held funds
        _, err = tx.Exec(context.Background(),
            `UPDATE wallets SET locked_amount = locked_amount - $1 WHERE user_id = $2 AND locked_amount >= $1`,
            amount, buyerID,
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to release held funds"})
        }
        // Log refund transaction
        _, err = tx.Exec(context.Background(),
            `INSERT INTO transactions (user_id, amount, type, status, reference, created_at)
             VALUES ($1, $2, 'credit', 'refunded', $3, $4)`,
            buyerID, amount, orderID, time.Now(),
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record refund"})
        }
    }

    _, err = tx.Exec(context.Background(),
        `UPDATE orders SET status = 'declined', updated_at = NOW() WHERE id = $1`,
        orderID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update order status"})
    }

    if err = tx.Commit(context.Background()); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "commit failed"})
    }

    return c.JSON(http.StatusOK, echo.Map{"message": "Order rejected"})
}

// =========================
// CompleteOrder - Buyer marks order complete (releases escrow funds)
// =========================
func CompleteOrder(c echo.Context) error {
	buyerID, ok := c.Get("user_id").(string)
	if !ok || buyerID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	orderID := c.Param("id")
	if orderID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id in URL"})
	}

    var sellerID string
    var amount int64
    err := db.Conn.QueryRow(context.Background(),
        `SELECT seller_id, amount FROM orders WHERE id = $1 AND buyer_id = $2 AND status IN ('in_progress','delivered')`,
        orderID, buyerID).Scan(&sellerID, &amount)
    if err != nil {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "order not found or not active"})
    }

    tx, err := db.Conn.Begin(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction start failed"})
	}
	defer tx.Rollback(context.Background())

    // Move funds from buyer escrow to seller balance
    _, err = tx.Exec(context.Background(),
        `UPDATE wallets SET escrow = escrow - $1 WHERE user_id = $2 AND escrow >= $1`,
        amount, buyerID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to deduct buyer escrow"})
    }

    _, err = tx.Exec(context.Background(),
        `UPDATE wallets SET balance = balance + $1 WHERE user_id = $2`,
        amount, sellerID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to credit seller"})
    }

    _, err = tx.Exec(context.Background(),
        `UPDATE orders SET status = 'completed', updated_at = NOW() WHERE id = $1`,
        orderID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update status"})
    }

    // Log transactions: buyer escrow release (debit) and seller credit
    _, err = tx.Exec(context.Background(),
        `INSERT INTO transactions (user_id, amount, type, status, reference, created_at)
         VALUES ($1, $2, 'credit', 'credited', $3, $4)`,
        sellerID, amount, orderID, time.Now(),
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record seller credit"})
    }

    if err = tx.Commit(context.Background()); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "commit failed"})
    }

    // Notify seller of completion/payout (best-effort)
    var sellerEmail string
    _ = db.Conn.QueryRow(context.Background(), `SELECT email FROM users WHERE id = $1`, sellerID).Scan(&sellerEmail)
    if sellerEmail != "" {
        _ = alerts.EnqueueOrderCompleted(orderID, buyerID, sellerID, sellerEmail, float64(amount))
    }

    return c.JSON(http.StatusOK, echo.Map{"message": "Order completed successfully"})
}

// =========================
// GetUserOrders - Fetch all orders for a user (as buyer or seller)
// =========================
func GetUserOrders(c echo.Context) error {
	uid, ok := c.Get("user_id").(string)
	if !ok || uid == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	rows, err := db.Conn.Query(context.Background(),
		`SELECT id, service_id, buyer_id, seller_id, amount, status, created_at
		 FROM orders WHERE buyer_id = $1 OR seller_id = $1 ORDER BY created_at DESC`, uid)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch orders"})
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.ServiceID, &o.BuyerID, &o.SellerID, &o.Amount, &o.Status, &o.CreatedAt); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to parse record"})
		}
		orders = append(orders, o)
	}

	return c.JSON(http.StatusOK, echo.Map{"orders": orders})
}

// =========================
// CancelOrder - Buyer cancels an order
// =========================
func CancelOrder(c echo.Context) error {
    buyerID, ok := c.Get("user_id").(string)
    if !ok || buyerID == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
    }

    orderID := c.Param("id")
    if orderID == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id in URL"})
    }

    var amount int64
    var status string
    var sellerID string
    err := db.Conn.QueryRow(context.Background(),
        `SELECT amount, status, seller_id FROM orders WHERE id = $1 AND buyer_id = $2`,
        orderID, buyerID,
    ).Scan(&amount, &status, &sellerID)
    if err != nil {
        return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found"})
    }

    // Only pending_acceptance, in_progress or delivered can be cancelled by buyer
    if status != "pending_acceptance" && status != "in_progress" && status != "delivered" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "order cannot be cancelled at this stage"})
    }

    tx, err := db.Conn.Begin(context.Background())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction start failed"})
    }
    defer tx.Rollback(context.Background())

    if status == "in_progress" || status == "delivered" {
        // Refund escrow to buyer
        _, err = tx.Exec(context.Background(),
            `UPDATE wallets SET escrow = escrow - $1, balance = balance + $1 WHERE user_id = $2 AND escrow >= $1`,
            amount, buyerID,
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to refund escrow"})
        }
        // Log refund transaction
        _, err = tx.Exec(context.Background(),
            `INSERT INTO transactions (user_id, amount, type, status, reference, created_at)
             VALUES ($1, $2, 'credit', 'refunded', $3, $4)`,
            buyerID, amount, orderID, time.Now(),
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record refund"})
        }
    } else if status == "pending_acceptance" {
        // Release held funds: decrease locked_amount
        _, err = tx.Exec(context.Background(),
            `UPDATE wallets SET locked_amount = locked_amount - $1 WHERE user_id = $2 AND locked_amount >= $1`,
            amount, buyerID,
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to release held funds"})
        }
        // Log refund transaction
        _, err = tx.Exec(context.Background(),
            `INSERT INTO transactions (user_id, amount, type, status, reference, created_at)
             VALUES ($1, $2, 'credit', 'refunded', $3, $4)`,
            buyerID, amount, orderID, time.Now(),
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record refund"})
        }
    }

    // Update order status
    _, err = tx.Exec(context.Background(),
        `UPDATE orders SET status = 'canceled', updated_at = NOW() WHERE id = $1`,
        orderID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update order status"})
    }

    if err = tx.Commit(context.Background()); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "commit failed"})
    }

    // Notify seller of cancellation (best-effort)
    var sellerEmail string
    _ = db.Conn.QueryRow(context.Background(), `SELECT email FROM users WHERE id = $1`, sellerID).Scan(&sellerEmail)
    if sellerEmail != "" {
        _ = alerts.EnqueueOrderCancelled(orderID, buyerID, sellerID, sellerEmail, float64(amount))
    }

    return c.JSON(http.StatusOK, echo.Map{"message": "Order cancelled"})
}

// =========================
// DeclineOrder - Seller declines after accepting/confirming
// =========================
func DeclineOrder(c echo.Context) error {
    sellerID, ok := c.Get("user_id").(string)
    if !ok || sellerID == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
    }

    orderID := c.Param("id")
    if orderID == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id in URL"})
    }

    var buyerID string
    var amount int64
    var status string
    err := db.Conn.QueryRow(context.Background(),
        `SELECT buyer_id, amount, status FROM orders WHERE id = $1 AND seller_id = $2`,
        orderID, sellerID,
    ).Scan(&buyerID, &amount, &status)
    if err != nil {
        return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found"})
    }

    if status != "in_progress" && status != "delivered" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "order not in declinable state"})
    }

    tx, err := db.Conn.Begin(context.Background())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction start failed"})
    }
    defer tx.Rollback(context.Background())

    if status == "in_progress" || status == "delivered" {
        // Refund escrow to buyer
        _, err = tx.Exec(context.Background(),
            `UPDATE wallets SET escrow = escrow - $1, balance = balance + $1 WHERE user_id = $2 AND escrow >= $1`,
            amount, buyerID,
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to refund escrow"})
        }
        // Log refund transaction
        _, err = tx.Exec(context.Background(),
            `INSERT INTO transactions (user_id, amount, type, status, reference, created_at)
             VALUES ($1, $2, 'credit', 'refunded', $3, $4)`,
            buyerID, amount, orderID, time.Now(),
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record refund"})
        }
    }

    // Update order status
    _, err = tx.Exec(context.Background(),
        `UPDATE orders SET status = 'declined', updated_at = NOW() WHERE id = $1`,
        orderID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update order status"})
    }

    if err = tx.Commit(context.Background()); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "commit failed"})
    }

    // Notify buyer of decline (best-effort)
    var buyerEmail string
    _ = db.Conn.QueryRow(context.Background(), `SELECT email FROM users WHERE id = $1`, buyerID).Scan(&buyerEmail)
    if buyerEmail != "" {
        _ = alerts.EnqueueOrderDeclined(orderID, buyerID, sellerID, buyerEmail, float64(amount))
    }

    return c.JSON(http.StatusOK, echo.Map{"message": "Order declined"})
}

// =========================
// DeliverOrder - Seller marks work delivered
// =========================
func DeliverOrder(c echo.Context) error {
    sellerID, ok := c.Get("user_id").(string)
    if !ok || sellerID == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
    }

    orderID := c.Param("id")
    if orderID == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id in URL"})
    }

    // Mark delivered
    res, err := db.Conn.Exec(context.Background(),
        `UPDATE orders SET status = 'delivered', updated_at = NOW() WHERE id = $1 AND seller_id = $2 AND status = 'in_progress'`,
        orderID, sellerID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update order status"})
    }
    if res.RowsAffected() == 0 {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "order not found or not confirmable"})
    }

    // Notify buyer of delivery (best-effort)
    var buyerID string
    var amount float64
    _ = db.Conn.QueryRow(context.Background(), `SELECT buyer_id, amount FROM orders WHERE id = $1`, orderID).Scan(&buyerID, &amount)
    var buyerEmail string
    _ = db.Conn.QueryRow(context.Background(), `SELECT email FROM users WHERE id = $1`, buyerID).Scan(&buyerEmail)
    if buyerEmail != "" {
        _ = alerts.EnqueueOrderDelivered(orderID, buyerID, sellerID, buyerEmail, amount)
    }

    return c.JSON(http.StatusOK, echo.Map{"message": "Order delivered"})
}
