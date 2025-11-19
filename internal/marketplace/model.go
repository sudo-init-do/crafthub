package marketplace

import "time"

// Service represents a service listed by a user
type Service struct {
    ID          string    `json:"id"`
    UserID      string    `json:"user_id"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Price       int64     `json:"price"`
    Category    string    `json:"category,omitempty"`
    DeliveryTimeDays int   `json:"delivery_time_days,omitempty"`
    Status      string    `json:"status,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
}

// ServiceSummary is used in discovery responses with aggregated fields
type ServiceSummary struct {
    ID          string    `json:"id"`
    UserID      string    `json:"user_id"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Price       int64     `json:"price"`
    Category    string    `json:"category,omitempty"`
    DeliveryTimeDays int   `json:"delivery_time_days,omitempty"`
    Status      string    `json:"status,omitempty"`
    AvgRating   float64   `json:"avg_rating"`
    CreatedAt   time.Time `json:"created_at"`
}

// Order represents an order placed by a user for a service
type Order struct {
    ID         string    `json:"id"`
    ServiceID  string    `json:"service_id"`
    BuyerID    string    `json:"buyer_id"`
    SellerID   string    `json:"seller_id"`
    Amount     int64     `json:"amount"`
    Status     string    `json:"status"` // pending_acceptance, in_progress, delivered, declined, completed, canceled
    CreatedAt  time.Time `json:"created_at"`
}
