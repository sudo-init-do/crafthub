package alerts

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

// ensureClient returns a usable client instance
func ensureClient() *asynq.Client {
	if client == nil {
		Init()
	}
	return client
}

// EnqueueWelcomeEmail schedules a welcome email to the user
func EnqueueWelcomeEmail(userID, email, name string) error {
	base := os.Getenv("APP_URL")
	if base == "" {
		base = "http://localhost:3000"
	}
	base = strings.TrimRight(base, "/")

	subject := fmt.Sprintf("Welcome to CraftHub, %s!", name)
	body := fmt.Sprintf("Hi %s, thanks for joining CraftHub.\n\nOpen CraftHub: %s\n\nIf the link doesn’t work, copy and paste the URL above.", name, base)

	env := EmailEnvelope{
		To:      email,
		Subject: subject,
		Body:    body,
	}
	payload := WelcomeEmailPayload{UserID: userID, Name: name, Email: email, Envelope: env, SentAt: time.Now()}
	b, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskWelcomeEmail, b)
	_, err := ensureClient().Enqueue(task, asynq.Queue("emails"))
	return err
}

// EnqueueBookingConfirmation notifies the buyer after seller confirms
func EnqueueBookingConfirmation(orderID, buyerID, sellerID, buyerEmail string, amount float64) error {
	env := EmailEnvelope{
		To:      buyerEmail,
		Subject: "Your booking has been confirmed",
		Body:    fmt.Sprintf("Order %s is confirmed. Amount %.2f.", orderID, amount),
	}
	payload := BookingConfirmationPayload{OrderID: orderID, BuyerID: buyerID, SellerID: sellerID, Email: buyerEmail, Amount: amount, Envelope: env, SentAt: time.Now()}
	b, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskBookingConfirmation, b)
	_, err := ensureClient().Enqueue(task, asynq.Queue("emails"))
	return err
}

// EnqueueAdminAlert sends an alert to admins (currently logs)
func EnqueueAdminAlert(adminID, severity, message string) error {
	env := EmailEnvelope{To: "admin@crafthub.local", Subject: "Admin Alert", Body: message}
	payload := AdminAlertPayload{AdminID: adminID, Severity: severity, Message: message, Envelope: env, SentAt: time.Now()}
	b, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskAdminAlert, b)
	_, err := ensureClient().Enqueue(task, asynq.Queue("alerts"))
	return err
}

// EnqueuePasswordReset schedules a password reset notification
func EnqueuePasswordReset(userID, email, resetURL, name string) error {
	expiry := os.Getenv("PASSWORD_RESET_EXP_MINUTES")
	if expiry == "" {
		expiry = "30"
	}
	subject := "Password reset instructions"
	body := fmt.Sprintf("Hello %s,\n\nWe received a request to reset your CraftHub password.\n\nTo proceed, open the link below:\n%s\n\nThis link expires in %s minutes. If you did not request this, no action is required.\n\nNeed help? Reply to this email.\n\n— CraftHub Team", name, resetURL, expiry)

	env := EmailEnvelope{To: email, Subject: subject, Body: body}
	payload := PasswordResetPayload{UserID: userID, Email: email, ResetURL: resetURL, Envelope: env, Requested: time.Now()}
	b, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskPasswordReset, b)
	_, err := ensureClient().Enqueue(task, asynq.Queue("emails"))
	return err
}

// EnqueueOrderCancelled notifies the seller that the buyer cancelled the order
func EnqueueOrderCancelled(orderID, buyerID, sellerID, sellerEmail string, amount float64) error {
	env := EmailEnvelope{
		To:      sellerEmail,
		Subject: "Order cancelled by buyer",
		Body:    fmt.Sprintf("Order %s was cancelled. Amount %.2f will be refunded if escrowed.", orderID, amount),
	}
	payload := OrderCancelledPayload{OrderID: orderID, BuyerID: buyerID, SellerID: sellerID, Email: sellerEmail, Amount: amount, Envelope: env, SentAt: time.Now()}
	b, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskOrderCancelled, b)
	_, err := ensureClient().Enqueue(task, asynq.Queue("emails"))
	return err
}

// EnqueueOrderDeclined notifies the buyer that the seller declined the order
func EnqueueOrderDeclined(orderID, buyerID, sellerID, buyerEmail string, amount float64) error {
	env := EmailEnvelope{
		To:      buyerEmail,
		Subject: "Order declined by seller",
		Body:    fmt.Sprintf("Order %s was declined by the seller. Amount %.2f will be refunded if escrowed.", orderID, amount),
	}
	payload := OrderDeclinedPayload{OrderID: orderID, BuyerID: buyerID, SellerID: sellerID, Email: buyerEmail, Amount: amount, Envelope: env, SentAt: time.Now()}
	b, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskOrderDeclined, b)
	_, err := ensureClient().Enqueue(task, asynq.Queue("emails"))
	return err
}

// EnqueueOrderDelivered notifies the buyer that the seller delivered the work
func EnqueueOrderDelivered(orderID, buyerID, sellerID, buyerEmail string, amount float64) error {
	env := EmailEnvelope{
		To:      buyerEmail,
		Subject: "Your order has been delivered",
		Body:    fmt.Sprintf("Order %s is delivered. Please review and complete to release payment.", orderID),
	}
	payload := OrderDeliveredPayload{OrderID: orderID, BuyerID: buyerID, SellerID: sellerID, Email: buyerEmail, Amount: amount, Envelope: env, SentAt: time.Now()}
	b, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskOrderDelivered, b)
	_, err := ensureClient().Enqueue(task, asynq.Queue("emails"))
	return err
}

// EnqueueOrderCompleted notifies the seller that the buyer completed the order (payout incoming)
func EnqueueOrderCompleted(orderID, buyerID, sellerID, sellerEmail string, amount float64) error {
	env := EmailEnvelope{
		To:      sellerEmail,
		Subject: "Order completed and paid",
		Body:    fmt.Sprintf("Order %s is completed. Amount %.2f has been released to your wallet.", orderID, amount),
	}
	payload := OrderCompletedPayload{OrderID: orderID, BuyerID: buyerID, SellerID: sellerID, Email: sellerEmail, Amount: amount, Envelope: env, SentAt: time.Now()}
	b, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskOrderCompleted, b)
	_, err := ensureClient().Enqueue(task, asynq.Queue("emails"))
	return err
}
