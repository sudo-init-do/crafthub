package alerts

import "time"

// Task type constants
const (
    TaskWelcomeEmail        = "email:welcome"
    TaskBookingConfirmation = "email:booking_confirmation"
    TaskAdminAlert          = "email:admin_alert"
    TaskPasswordReset       = "email:password_reset"
    TaskOrderCancelled      = "email:order_cancelled"
    TaskOrderDeclined       = "email:order_declined"
    TaskOrderDelivered      = "email:order_delivered"
    TaskOrderCompleted      = "email:order_completed"
    TaskMessageNew          = "email:message_new"
)

// Common envelope for email-like notifications
type EmailEnvelope struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
    Body    string `json:"body"`
}

// Welcome email payload
type WelcomeEmailPayload struct {
    UserID   string       `json:"user_id"`
    Name     string       `json:"name"`
    Email    string       `json:"email"`
    Envelope EmailEnvelope `json:"envelope"`
    SentAt   time.Time    `json:"sent_at"`
}

// Booking confirmation payload
type BookingConfirmationPayload struct {
    OrderID string       `json:"order_id"`
    BuyerID string       `json:"buyer_id"`
    SellerID string      `json:"seller_id"`
    Email   string       `json:"email"`
    Amount  float64      `json:"amount"`
    Envelope EmailEnvelope `json:"envelope"`
    SentAt  time.Time    `json:"sent_at"`
}

// Admin alert payload
type AdminAlertPayload struct {
    AdminID  string       `json:"admin_id"`
    Severity string       `json:"severity"` // info|warning|critical
    Message  string       `json:"message"`
    Envelope EmailEnvelope `json:"envelope"`
    SentAt   time.Time    `json:"sent_at"`
}

// Password reset payload
type PasswordResetPayload struct {
    UserID    string       `json:"user_id"`
    Email     string       `json:"email"`
    ResetURL  string       `json:"reset_url"`
    Envelope  EmailEnvelope `json:"envelope"`
    Requested time.Time    `json:"requested"`
}

// Order cancelled payload (sent to seller)
type OrderCancelledPayload struct {
    OrderID  string        `json:"order_id"`
    BuyerID  string        `json:"buyer_id"`
    SellerID string        `json:"seller_id"`
    Email    string        `json:"email"`
    Amount   float64       `json:"amount"`
    Envelope EmailEnvelope `json:"envelope"`
    SentAt   time.Time     `json:"sent_at"`
}

// Order declined payload (sent to buyer)
type OrderDeclinedPayload struct {
    OrderID  string        `json:"order_id"`
    BuyerID  string        `json:"buyer_id"`
    SellerID string        `json:"seller_id"`
    Email    string        `json:"email"`
    Amount   float64       `json:"amount"`
    Envelope EmailEnvelope `json:"envelope"`
    SentAt   time.Time     `json:"sent_at"`
}

// Order delivered payload (sent to buyer)
type OrderDeliveredPayload struct {
    OrderID  string        `json:"order_id"`
    BuyerID  string        `json:"buyer_id"`
    SellerID string        `json:"seller_id"`
    Email    string        `json:"email"`
    Amount   float64       `json:"amount"`
    Envelope EmailEnvelope `json:"envelope"`
    SentAt   time.Time     `json:"sent_at"`
}

// Order completed payload (sent to seller)
type OrderCompletedPayload struct {
    OrderID  string        `json:"order_id"`
    BuyerID  string        `json:"buyer_id"`
    SellerID string        `json:"seller_id"`
    Email    string        `json:"email"`
    Amount   float64       `json:"amount"`
    Envelope EmailEnvelope `json:"envelope"`
    SentAt   time.Time     `json:"sent_at"`
}

// Message new payload (sent to recipient on new message)
type MessageNewPayload struct {
    OrderID   string        `json:"order_id"`
    SenderID  string        `json:"sender_id"`
    Recipient string        `json:"recipient"`
    Email     string        `json:"email"`
    Body      string        `json:"body"`
    Envelope  EmailEnvelope `json:"envelope"`
    SentAt    time.Time     `json:"sent_at"`
}
