package messaging

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/alerts"
	"github.com/sudo-init-do/crafthub/internal/db"
)

// SendMessage - buyer or seller sends a message in an order thread
func SendMessage(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	orderID := c.Param("id")
	if orderID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id"})
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := c.Bind(&body); err != nil || body.Content == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid payload"})
	}

	// Ensure user is buyer or seller of this order and derive recipient
	var buyerID, sellerID string
	err := db.Conn.QueryRow(context.Background(),
		`SELECT buyer_id, seller_id FROM orders WHERE id = $1`, orderID,
	).Scan(&buyerID, &sellerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch order"})
	}

	var recipientID string
	switch userID {
	case buyerID:
		recipientID = sellerID
	case sellerID:
		recipientID = buyerID
	default:
		return c.JSON(http.StatusForbidden, echo.Map{"error": "not a participant in this order"})
	}

	msgID := uuid.New().String()
	var createdAt time.Time
	err = db.Conn.QueryRow(context.Background(),
		`INSERT INTO messages (id, order_id, sender_id, recipient_id, content)
         VALUES ($1, $2, $3, $4, $5) RETURNING created_at`,
		msgID, orderID, userID, recipientID, body.Content,
	).Scan(&createdAt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to send message"})
	}

	// Broadcast new message event to WS subscribers
	BroadcastNewMessage(orderID, echo.Map{
		"id":           msgID,
		"order_id":     orderID,
		"sender_id":    userID,
		"recipient_id": recipientID,
		"content":      body.Content,
		"created_at":   createdAt.UTC().Format(time.RFC3339),
	})

	// In-app notification for recipient
	notifTitle := "New message on your order"
	ref := msgID
	meta := "{}"
	_ = alerts.CreateNotification(recipientID, "message:new", notifTitle, body.Content, &ref, &meta)

	// Email notification (best-effort)
	var recipientEmail string
	_ = db.Conn.QueryRow(context.Background(), `SELECT email FROM users WHERE id = $1`, recipientID).Scan(&recipientEmail)
	if recipientEmail != "" {
		_ = alerts.EnqueueMessageNew(orderID, userID, recipientEmail, recipientID, body.Content)
	}

	return c.JSON(http.StatusOK, echo.Map{"message_id": msgID})
}

// ListMessages - get the conversation for an order
func ListMessages(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	orderID := c.Param("id")
	if orderID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id"})
	}

	// Verify participation
	var buyerID, sellerID string
	err := db.Conn.QueryRow(context.Background(),
		`SELECT buyer_id, seller_id FROM orders WHERE id = $1`, orderID,
	).Scan(&buyerID, &sellerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch order"})
	}
	if userID != buyerID && userID != sellerID {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "not a participant in this order"})
	}

	// Optional since filter for incremental fetches
	sinceStr := c.QueryParam("since")
	var rows pgx.Rows
	if sinceStr != "" {
		sinceTime, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid since timestamp, use RFC3339"})
		}
		rows, err = db.Conn.Query(context.Background(),
			`SELECT id, sender_id, recipient_id, content, created_at, read_at
             FROM messages WHERE order_id = $1 AND created_at > $2 ORDER BY created_at ASC`, orderID, sinceTime,
		)
	} else {
		rows, err = db.Conn.Query(context.Background(),
			`SELECT id, sender_id, recipient_id, content, created_at, read_at
             FROM messages WHERE order_id = $1 ORDER BY created_at ASC`, orderID,
		)
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to list messages"})
	}
	defer rows.Close()

	type message struct {
		ID          string      `json:"id"`
		SenderID    string      `json:"sender_id"`
		RecipientID string      `json:"recipient_id"`
		Content     string      `json:"content"`
		CreatedAt   string      `json:"created_at"`
		ReadAt      interface{} `json:"read_at"`
	}

	var msgs []message
	for rows.Next() {
		var m message
		var readAt sql.NullTime
		var createdAt sql.NullTime
		if err := rows.Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.Content, &createdAt, &readAt); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to parse record"})
		}
		if createdAt.Valid {
			m.CreatedAt = createdAt.Time.UTC().Format(time.RFC3339)
		}
		if readAt.Valid {
			m.ReadAt = readAt.Time.UTC().Format(time.RFC3339)
		} else {
			m.ReadAt = nil
		}
		msgs = append(msgs, m)
	}

	return c.JSON(http.StatusOK, echo.Map{"messages": msgs})
}

// UnreadCount - get unread count for the current user in an order thread
func UnreadCount(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	orderID := c.Param("id")
	if orderID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id"})
	}

	// Verify participation
	var buyerID, sellerID string
	err := db.Conn.QueryRow(context.Background(),
		`SELECT buyer_id, seller_id FROM orders WHERE id = $1`, orderID,
	).Scan(&buyerID, &sellerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch order"})
	}
	if userID != buyerID && userID != sellerID {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "not a participant in this order"})
	}

	var count int64
	err = db.Conn.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM messages WHERE order_id = $1 AND recipient_id = $2 AND read_at IS NULL`,
		orderID, userID,
	).Scan(&count)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to compute unread count"})
	}

	return c.JSON(http.StatusOK, echo.Map{"unread": count})
}

// MarkMessageRead - recipient marks a specific message as read
func MarkMessageRead(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	orderID := c.Param("id")
	msgID := c.Param("message_id")
	if orderID == "" || msgID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order or message id"})
	}

	// Ensure message belongs to the order and user is recipient
	var recipientID string
	err := db.Conn.QueryRow(context.Background(),
		`SELECT recipient_id FROM messages WHERE id = $1 AND order_id = $2`, msgID, orderID,
	).Scan(&recipientID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "message not found"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch message"})
	}
	if recipientID != userID {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "not the recipient"})
	}

	var readTS time.Time
	err = db.Conn.QueryRow(context.Background(),
		`UPDATE messages SET read_at = NOW() WHERE id = $1 AND recipient_id = $2 RETURNING read_at`, msgID, userID,
	).Scan(&readTS)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to mark read"})
	}

	// Broadcast read event
	BroadcastMessageRead(orderID, echo.Map{
		"message_id": msgID,
		"order_id":   orderID,
		"user_id":    userID,
		"read_at":    readTS.UTC().Format(time.RFC3339),
	})

	return c.JSON(http.StatusOK, echo.Map{"message_id": msgID, "read_at": readTS.UTC().Format(time.RFC3339)})
}
