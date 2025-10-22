package marketplace

import "time"

// Service represents a service listed by a user
type Service struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	CreatedAt   time.Time `json:"created_at"`
}

// Order represents an order placed by a user for a service
type Order struct {
	ID         string    `json:"id"`
	ServiceID  string    `json:"service_id"`
	BuyerID    string    `json:"buyer_id"`
	SellerID   string    `json:"seller_id"`
	Amount     float64   `json:"amount"`
	Status     string    `json:"status"` // pending, completed, cancelled
	CreatedAt  time.Time `json:"created_at"`
}
