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
	var price float64
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

    var balance float64
    err = db.Conn.QueryRow(context.Background(),
        `SELECT balance FROM wallets WHERE user_id = $1`,
        buyerID,
    ).Scan(&balance)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "wallet not found"})
    }

    if balance < price {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "insufficient balance"})
    }

    // Do not deduct funds yet; just create a pending order.
    orderID := uuid.New().String()
    _, err = db.Conn.Exec(context.Background(),
        `INSERT INTO orders (id, service_id, buyer_id, seller_id, amount, status, created_at)
         VALUES ($1, $2, $3, $4, $5, 'pending', $6)`,
        orderID, req.ServiceID, buyerID, sellerID, price, time.Now(),
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to create order"})
    }

    return c.JSON(http.StatusCreated, echo.Map{
        "order_id": orderID,
        "message":  "Order placed successfully. Awaiting seller acceptance.",
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
    var amount float64
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

    if status != "pending" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "order not pending"})
    }

    // Begin transaction: move buyer balance -> escrow, set status confirmed
    tx, err := db.Conn.Begin(context.Background())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction start failed"})
    }
    defer tx.Rollback(context.Background())

    // Ensure buyer can cover amount
    var balance float64
    err = tx.QueryRow(context.Background(), `SELECT balance FROM wallets WHERE user_id = $1 FOR UPDATE`, buyerID).Scan(&balance)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "buyer wallet not found"})
    }
    if balance < amount {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "buyer has insufficient balance"})
    }

    // Move funds into escrow
    _, err = tx.Exec(context.Background(),
        `UPDATE wallets SET balance = balance - $1, escrow = escrow + $1 WHERE user_id = $2`,
        amount, buyerID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to move funds to escrow"})
    }

    // Update order status
    _, err = tx.Exec(context.Background(),
        `UPDATE orders SET status = 'confirmed', updated_at = NOW() WHERE id = $1`,
        orderID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update order status"})
    }

    // Log escrow hold transaction
    _, err = tx.Exec(context.Background(),
        `INSERT INTO transactions (user_id, amount, type, status, reference, created_at)
         VALUES ($1, $2, 'debit', 'escrow_hold', $3, $4)`,
        buyerID, amount, orderID, time.Now(),
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record escrow hold"})
    }

    if err = tx.Commit(context.Background()); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "commit failed"})
    }

    return c.JSON(http.StatusOK, echo.Map{"message": "Order accepted and escrowed"})
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
    var amount float64
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

    if status == "confirmed" {
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
             VALUES ($1, $2, 'credit', 'refund', $3, $4)`,
            buyerID, amount, orderID, time.Now(),
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record refund"})
        }
    }

    _, err = tx.Exec(context.Background(),
        `UPDATE orders SET status = 'rejected', updated_at = NOW() WHERE id = $1`,
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
	var amount float64
    err := db.Conn.QueryRow(context.Background(),
        `SELECT seller_id, amount FROM orders WHERE id = $1 AND buyer_id = $2 AND status IN ('confirmed','delivered')`,
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
         VALUES ($1, $2, 'debit', 'escrow_release', $3, $4)`,
        buyerID, amount, orderID, time.Now(),
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record buyer escrow release"})
    }

    _, err = tx.Exec(context.Background(),
        `INSERT INTO transactions (user_id, amount, type, status, reference, created_at)
         VALUES ($1, $2, 'credit', 'success', $3, $4)`,
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
        _ = alerts.EnqueueOrderCompleted(orderID, buyerID, sellerID, sellerEmail, amount)
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

    var amount float64
    var status string
    var sellerID string
    err := db.Conn.QueryRow(context.Background(),
        `SELECT amount, status, seller_id FROM orders WHERE id = $1 AND buyer_id = $2`,
        orderID, buyerID,
    ).Scan(&amount, &status, &sellerID)
    if err != nil {
        return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found"})
    }

    // Only pending or confirmed can be cancelled by buyer
    if status != "pending" && status != "confirmed" && status != "active" && status != "delivered" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "order cannot be cancelled at this stage"})
    }

    tx, err := db.Conn.Begin(context.Background())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction start failed"})
    }
    defer tx.Rollback(context.Background())

    if status == "confirmed" || status == "delivered" {
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
             VALUES ($1, $2, 'credit', 'refund', $3, $4)`,
            buyerID, amount, orderID, time.Now(),
        )
        if err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to record refund"})
        }
    }

    // Update order status
    _, err = tx.Exec(context.Background(),
        `UPDATE orders SET status = 'cancelled', updated_at = NOW() WHERE id = $1`,
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
        _ = alerts.EnqueueOrderCancelled(orderID, buyerID, sellerID, sellerEmail, amount)
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
    var amount float64
    var status string
    err := db.Conn.QueryRow(context.Background(),
        `SELECT buyer_id, amount, status FROM orders WHERE id = $1 AND seller_id = $2`,
        orderID, sellerID,
    ).Scan(&buyerID, &amount, &status)
    if err != nil {
        return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found"})
    }

    if status != "confirmed" && status != "delivered" && status != "active" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "order not in declinable state"})
    }

    tx, err := db.Conn.Begin(context.Background())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction start failed"})
    }
    defer tx.Rollback(context.Background())

    if status == "confirmed" || status == "delivered" {
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
             VALUES ($1, $2, 'credit', 'refund', $3, $4)`,
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
        _ = alerts.EnqueueOrderDeclined(orderID, buyerID, sellerID, buyerEmail, amount)
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
        `UPDATE orders SET status = 'delivered', updated_at = NOW() WHERE id = $1 AND seller_id = $2 AND status = 'confirmed'`,
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
