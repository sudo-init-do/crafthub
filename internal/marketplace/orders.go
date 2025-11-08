package marketplace

import (
    "context"
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
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
		"message":  "Order placed successfully. Awaiting seller confirmation.",
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

	res, err := db.Conn.Exec(context.Background(),
		`UPDATE orders SET status = 'active', updated_at = NOW()
		 WHERE id = $1 AND seller_id = $2 AND status = 'pending'`,
		orderID, sellerID)
	if err != nil {
		println("AcceptOrder SQL error:", err.Error())
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update order"})
	}

	rows := res.RowsAffected()
	println("AcceptOrder rows affected:", rows)

	if rows == 0 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "order not found or not pending"})
	}

	return c.JSON(http.StatusOK, echo.Map{"message": "Order accepted successfully"})
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

    var buyerID string
    var amount float64
    err := db.Conn.QueryRow(context.Background(),
        `SELECT buyer_id, amount FROM orders WHERE id = $1 AND seller_id = $2 AND status = 'pending'`,
        orderID, sellerID).Scan(&buyerID, &amount)
    if err != nil {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "order not found or already handled"})
    }

    // No funds were deducted in 'pending' state, so just mark rejected.
    _, err = db.Conn.Exec(context.Background(),
        `UPDATE orders SET status = 'rejected', updated_at = NOW() WHERE id = $1`,
        orderID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update order status"})
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
        `SELECT seller_id, amount FROM orders WHERE id = $1 AND buyer_id = $2 AND status IN ('active','confirmed')`,
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

	if err = tx.Commit(context.Background()); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "commit failed"})
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
