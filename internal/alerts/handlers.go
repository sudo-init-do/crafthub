package alerts

import (
    "context"
    "net/http"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

// ListNotifications returns current user's notifications, newest first
func ListNotifications(c echo.Context) error {
    userID, ok := c.Get("user_id").(string)
    if !ok || userID == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
    }

    rows, err := db.Conn.Query(context.Background(),
        `SELECT id::text, type, title, body, reference::text, metadata::text, created_at, read_at
         FROM notifications WHERE user_id = $1 ORDER BY created_at DESC`, userID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to load notifications"})
    }
    defer rows.Close()

    var items []map[string]interface{}
    for rows.Next() {
        var id, ntype, title, body, reference, metadata string
        var createdAt time.Time
        var readAt *time.Time
        if err := rows.Scan(&id, &ntype, &title, &body, &reference, &metadata, &createdAt, &readAt); err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to parse notification"})
        }
        item := map[string]interface{}{
            "id": id,
            "type": ntype,
            "title": title,
            "body": body,
            "reference": reference,
            "metadata": metadata,
            "created_at": createdAt.UTC().Format(time.RFC3339),
        }
        if readAt != nil {
            item["read_at"] = readAt.UTC().Format(time.RFC3339)
        } else {
            item["read_at"] = nil
        }
        items = append(items, item)
    }
    return c.JSON(http.StatusOK, echo.Map{"notifications": items})
}

// MarkNotificationRead marks specific notification as read
func MarkNotificationRead(c echo.Context) error {
    userID, ok := c.Get("user_id").(string)
    if !ok || userID == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
    }
    nid := c.Param("id")
    if nid == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing notification id"})
    }

    res, err := db.Conn.Exec(context.Background(),
        `UPDATE notifications SET read_at = NOW() WHERE id = $1 AND user_id = $2 AND read_at IS NULL`, nid, userID,
    )
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update"})
    }
    if res.RowsAffected() == 0 {
        return c.JSON(http.StatusNotFound, echo.Map{"error": "not found or already read"})
    }
    return c.JSON(http.StatusOK, echo.Map{"message": "ok"})
}

// CreateNotification inserts a notification item
func CreateNotification(userID, ntype, title, body string, reference *string, metadataJSON *string) error {
    _, err := db.Conn.Exec(context.Background(),
        `INSERT INTO notifications (user_id, type, title, body, reference, metadata)
         VALUES ($1, $2, $3, $4, $5, $6)`, userID, ntype, title, body, reference, metadataJSON,
    )
    return err
}
