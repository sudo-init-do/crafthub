package messaging

import (
    "context"
    "encoding/json"
    "net/http"
    "sync"

    "github.com/gorilla/websocket"
    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

type wsEvent struct {
    Type string      `json:"type"`
    Data interface{} `json:"data"`
}

type hub struct {
    orderID   string
    clients   map[*websocket.Conn]bool
    mu        sync.RWMutex
}

var (
    hubsMu sync.RWMutex
    hubs   = make(map[string]*hub)
)

func getHub(orderID string) *hub {
    hubsMu.Lock()
    defer hubsMu.Unlock()
    if h, ok := hubs[orderID]; ok {
        return h
    }
    h := &hub{orderID: orderID, clients: make(map[*websocket.Conn]bool)}
    hubs[orderID] = h
    return h
}

func (h *hub) broadcast(evt wsEvent) {
    payload, _ := json.Marshal(evt)
    h.mu.RLock()
    defer h.mu.RUnlock()
    for c := range h.clients {
        _ = c.WriteMessage(websocket.TextMessage, payload)
    }
}

func (h *hub) register(c *websocket.Conn) {
    h.mu.Lock()
    h.clients[c] = true
    h.mu.Unlock()
}

func (h *hub) unregister(c *websocket.Conn) {
    h.mu.Lock()
    if _, ok := h.clients[c]; ok {
        delete(h.clients, c)
    }
    h.mu.Unlock()
}

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool { return true },
}

// OrderWS - websocket for realtime updates on an order thread
func OrderWS(c echo.Context) error {
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
        return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found or inaccessible"})
    }
    if userID != buyerID && userID != sellerID {
        return c.JSON(http.StatusForbidden, echo.Map{"error": "not a participant in this order"})
    }

    ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
    if err != nil {
        return err
    }

    h := getHub(orderID)
    h.register(ws)

    // Optionally send a hello event
    h.broadcast(wsEvent{Type: "presence_join", Data: echo.Map{"user_id": userID}})

    // Read loop (discard client messages; protocol is server push for now)
    for {
        if _, _, err := ws.ReadMessage(); err != nil {
            h.unregister(ws)
            _ = ws.Close()
            h.broadcast(wsEvent{Type: "presence_leave", Data: echo.Map{"user_id": userID}})
            break
        }
    }
    return nil
}

// BroadcastNewMessage - publish a new message event to order hub
func BroadcastNewMessage(orderID string, message interface{}) {
    h := getHub(orderID)
    h.broadcast(wsEvent{Type: "message_new", Data: message})
}

// BroadcastMessageRead - publish a message read event
func BroadcastMessageRead(orderID string, payload interface{}) {
    h := getHub(orderID)
    h.broadcast(wsEvent{Type: "message_read", Data: payload})
}

