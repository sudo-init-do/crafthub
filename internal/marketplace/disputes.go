package marketplace

import (
    "context"
    "database/sql"
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/alerts"
    "github.com/sudo-init-do/crafthub/internal/db"
)

// OpenDispute allows a buyer or seller to open a dispute against an order
// POST /marketplace/orders/:id/dispute
func OpenDispute(c echo.Context) error {
    uid, ok := c.Get("user_id").(string)
    if !ok || uid == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
    }

    orderID := c.Param("id")
    if orderID == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id"})
    }

    var req struct { Reason string `json:"reason"` }
    if err := c.Bind(&req); err != nil || req.Reason == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid payload: reason required"})
    }

    // Verify participation
    var buyerID, sellerID string
    if err := db.Conn.QueryRow(context.Background(), `SELECT buyer_id, seller_id FROM orders WHERE id = $1`, orderID).Scan(&buyerID, &sellerID); err != nil {
        if err == sql.ErrNoRows {
            return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found"})
        }
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch order"})
    }
    if uid != buyerID && uid != sellerID {
        return c.JSON(http.StatusForbidden, echo.Map{"error": "not a participant in this order"})
    }

    disputeID := uuid.New().String()
    var createdAt time.Time
    if err := db.Conn.QueryRow(context.Background(),
        `INSERT INTO disputes (id, order_id, filer_id, reason) VALUES ($1, $2, $3, $4) RETURNING created_at`,
        disputeID, orderID, uid, req.Reason,
    ).Scan(&createdAt); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not open dispute"})
    }

    // Notify other participant and admins (best-effort)
    other := buyerID
    if uid == buyerID { other = sellerID }
    notifTitle := "Dispute opened on your order"
    ref := disputeID
    meta := "{}"
    _ = alerts.CreateNotification(other, "dispute:opened", notifTitle, req.Reason, &ref, &meta)
    _ = alerts.EnqueueAdminAlert(uid, "info", "New dispute opened: order "+orderID)

    return c.JSON(http.StatusCreated, echo.Map{"dispute_id": disputeID, "created_at": createdAt.UTC().Format(time.RFC3339)})
}

