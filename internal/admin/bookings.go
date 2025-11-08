package admin

import (
    "context"
    "net/http"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

type AdminOrder struct {
    ID        string    `json:"id"`
    BuyerID   string    `json:"buyer_id"`
    SellerID  string    `json:"seller_id"`
    ServiceID string    `json:"service_id"`
    Amount    int64     `json:"amount"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// GET /admin/bookings
func ListBookings(c echo.Context) error {
    rows, err := db.Conn.Query(context.Background(),
        `SELECT id, buyer_id, seller_id, service_id, amount, status, created_at, updated_at FROM orders ORDER BY created_at DESC`,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch bookings"})
    }
    defer rows.Close()

    var orders []AdminOrder
    for rows.Next() {
        var o AdminOrder
        if err := rows.Scan(&o.ID, &o.BuyerID, &o.SellerID, &o.ServiceID, &o.Amount, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to read booking record"})
        }
        orders = append(orders, o)
    }
    return c.JSON(http.StatusOK, echo.Map{"bookings": orders})
}
