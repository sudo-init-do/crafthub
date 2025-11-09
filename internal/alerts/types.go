package alerts

import "time"

// Task type constants
const (
    TaskWelcomeEmail        = "email:welcome"
    TaskBookingConfirmation = "email:booking_confirmation"
    TaskAdminAlert          = "email:admin_alert"
    TaskPasswordReset       = "email:password_reset"
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

