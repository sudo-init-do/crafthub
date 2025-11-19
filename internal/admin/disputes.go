package admin

import (
    "context"
    "net/http"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/alerts"
    "github.com/sudo-init-do/crafthub/internal/db"
)

type AdminDispute struct {
    ID         string  `json:"id"`
    OrderID    string  `json:"order_id"`
    FilerID    string  `json:"filer_id"`
    Reason     string  `json:"reason"`
    Status     string  `json:"status"`
    Resolution string  `json:"resolution"`
    Notes      string  `json:"notes"`
    CreatedAt  string  `json:"created_at"`
    ResolvedAt *string `json:"resolved_at"`
}

// GET /admin/disputes
func ListDisputes(c echo.Context) error {
    rows, err := db.Conn.Query(context.Background(),
        `SELECT id::text, order_id::text, filer_id::text, reason, status, COALESCE(resolution,'') AS resolution, COALESCE(notes,'') AS notes, created_at, resolved_at
         FROM disputes ORDER BY created_at DESC`,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch disputes"})
    }
    defer rows.Close()

    var items []AdminDispute
    for rows.Next() {
        var d AdminDispute
        var created time.Time
        var resolved *time.Time
        if err := rows.Scan(&d.ID, &d.OrderID, &d.FilerID, &d.Reason, &d.Status, &d.Resolution, &d.Notes, &created, &resolved); err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to read dispute record"})
        }
        d.CreatedAt = created.UTC().Format(time.RFC3339)
        if resolved != nil {
            s := resolved.UTC().Format(time.RFC3339)
            d.ResolvedAt = &s
        }
        items = append(items, d)
    }
    return c.JSON(http.StatusOK, echo.Map{"disputes": items})
}

// POST /admin/disputes/:id/resolve
func ResolveDispute(c echo.Context) error {
    adminID, ok := c.Get("user_id").(string)
    if !ok || adminID == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
    }
    id := c.Param("id")
    if id == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "dispute id required"})
    }
    var req struct {
        Resolution string `json:"resolution"` // refund|release|none
        Notes      string `json:"notes"`
    }
    if err := c.Bind(&req); err != nil || req.Resolution == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid payload: resolution required"})
    }
    if req.Resolution != "refund" && req.Resolution != "release" && req.Resolution != "none" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid resolution"})
    }

    // Update dispute
    var orderID, filerID string
    err := db.Conn.QueryRow(context.Background(), `UPDATE disputes SET status = 'resolved', resolution = $1, notes = $2, resolved_by = $3, resolved_at = NOW() WHERE id = $4 RETURNING order_id::text, filer_id::text`, req.Resolution, req.Notes, adminID, id).Scan(&orderID, &filerID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to resolve dispute"})
    }

    // Notify participants
    var buyerID, sellerID string
    _ = db.Conn.QueryRow(context.Background(), `SELECT buyer_id::text, seller_id::text FROM orders WHERE id = $1`, orderID).Scan(&buyerID, &sellerID)
    title := "Dispute resolved"
    meta := "{}"
    ref := id
    _ = alerts.CreateNotification(buyerID, "dispute:resolved", title, req.Resolution+" - "+req.Notes, &ref, &meta)
    _ = alerts.CreateNotification(sellerID, "dispute:resolved", title, req.Resolution+" - "+req.Notes, &ref, &meta)

    return c.JSON(http.StatusOK, echo.Map{"message": "resolved", "dispute_id": id, "resolution": req.Resolution})
}
