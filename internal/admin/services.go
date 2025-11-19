package admin

import (
    "context"
    "net/http"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

type AdminService struct {
    ID               string  `json:"id"`
    UserID           string  `json:"user_id"`
    Title            string  `json:"title"`
    Price            int64   `json:"price"`
    Category         string  `json:"category"`
    DeliveryTimeDays int     `json:"delivery_time_days"`
    Status           string  `json:"status"`
    CreatedAt        string  `json:"created_at"`
}

// GET /admin/services
func ListServices(c echo.Context) error {
    rows, err := db.Conn.Query(context.Background(),
        `SELECT id, user_id, title, price, category, delivery_time_days, status, created_at
         FROM services ORDER BY created_at DESC`,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch services"})
    }
    defer rows.Close()

    var items []AdminService
    for rows.Next() {
        var s AdminService
        var createdAt time.Time
        if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.Price, &s.Category, &s.DeliveryTimeDays, &s.Status, &createdAt); err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to read service record"})
        }
        s.CreatedAt = createdAt.UTC().Format(time.RFC3339)
        items = append(items, s)
    }
    return c.JSON(http.StatusOK, echo.Map{"services": items})
}

// POST /admin/services/:id/suspend
func SuspendService(c echo.Context) error {
    id := c.Param("id")
    if id == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "service id required"})
    }
    _, err := db.Conn.Exec(context.Background(), `UPDATE services SET status = 'suspended' WHERE id = $1`, id)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to suspend service"})
    }
    return c.JSON(http.StatusOK, echo.Map{"message": "service suspended", "service_id": id})
}

// POST /admin/services/:id/approve
func ApproveService(c echo.Context) error {
    id := c.Param("id")
    if id == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "service id required"})
    }
    _, err := db.Conn.Exec(context.Background(), `UPDATE services SET status = 'active' WHERE id = $1`, id)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to approve service"})
    }
    return c.JSON(http.StatusOK, echo.Map{"message": "service approved", "service_id": id})
}
