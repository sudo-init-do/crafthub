package alerts

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/hibiken/asynq"
)

var (
	client *asynq.Client
	server *asynq.Server
)

// Init starts the Asynq server and initializes a shared client.
func Init() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		// Prefer docker hostname, fallback to localhost
		if host := os.Getenv("REDIS_HOST"); host != "" {
			port := os.Getenv("REDIS_PORT")
			if port == "" {
				port = "6379"
			}
			redisAddr = host + ":" + port
		} else {
			// Default to docker-compose service name if running in container; otherwise localhost
			redisAddr = "redis:6379"
			if os.Getenv("RUN_LOCAL") == "true" {
				redisAddr = "127.0.0.1:6379"
			}
		}
	}

	opts := asynq.RedisClientOpt{Addr: redisAddr}
	client = asynq.NewClient(opts)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskWelcomeEmail, handleWelcomeEmail)
	mux.HandleFunc(TaskBookingConfirmation, handleBookingConfirmation)
	mux.HandleFunc(TaskAdminAlert, handleAdminAlert)
	mux.HandleFunc(TaskPasswordReset, handlePasswordReset)
	mux.HandleFunc(TaskOrderCancelled, handleOrderCancelled)
	mux.HandleFunc(TaskOrderDeclined, handleOrderDeclined)
	mux.HandleFunc(TaskOrderDelivered, handleOrderDelivered)
	mux.HandleFunc(TaskOrderCompleted, handleOrderCompleted)

	server = asynq.NewServer(opts, asynq.Config{
		Concurrency: 5,
		Queues: map[string]int{
			"emails": 10,
			"alerts": 5,
		},
	})
	go func() {
		if err := server.Run(mux); err != nil {
			log.Printf("Asynq server stopped: %v", err)
		}
	}()

	log.Printf("Asynq initialized (addr=%s)", redisAddr)
}

// Close releases client and stops server.
func Close() {
	if client != nil {
		_ = client.Close()
	}
	if server != nil {
		server.Shutdown()
	}
}

// Handlers below parse payloads and simulate email/push with logs.

func handleWelcomeEmail(_ context.Context, t *asynq.Task) error {
	var p WelcomeEmailPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	if err := SendEmail(p.Email, p.Envelope.Subject, p.Envelope.Body); err != nil {
		log.Printf("[notify][ERROR] WelcomeEmail send failed: %v", err)
		return err
	}
	log.Printf("[notify] WelcomeEmail sent -> to=%s user=%s", p.Email, p.UserID)
	return nil
}

func handleBookingConfirmation(_ context.Context, t *asynq.Task) error {
	var p BookingConfirmationPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	if err := SendEmail(p.Email, p.Envelope.Subject, p.Envelope.Body); err != nil {
		log.Printf("[notify][ERROR] BookingConfirmation send failed: %v", err)
		return err
	}
	log.Printf("[notify] BookingConfirmation sent -> order=%s to=%s", p.OrderID, p.Email)
	return nil
}

func handleAdminAlert(_ context.Context, t *asynq.Task) error {
	var p AdminAlertPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	if err := SendEmail(p.Envelope.To, p.Envelope.Subject, p.Envelope.Body); err != nil {
		log.Printf("[notify][ERROR] AdminAlert send failed: %v", err)
		return err
	}
	log.Printf("[notify] AdminAlert sent -> severity=%s by=%s", p.Severity, p.AdminID)
	return nil
}

func handlePasswordReset(_ context.Context, t *asynq.Task) error {
	var p PasswordResetPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	if err := SendEmail(p.Email, p.Envelope.Subject, p.Envelope.Body); err != nil {
		log.Printf("[notify][ERROR] PasswordReset send failed: %v", err)
		return err
	}
	log.Printf("[notify] PasswordReset sent -> to=%s", p.Email)
	return nil
}

func handleOrderCancelled(_ context.Context, t *asynq.Task) error {
	var p OrderCancelledPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	if err := SendEmail(p.Email, p.Envelope.Subject, p.Envelope.Body); err != nil {
		return err
	}
	log.Printf("[notify] OrderCancelled sent -> order=%s to=%s", p.OrderID, p.Email)
	return nil
}

func handleOrderDeclined(_ context.Context, t *asynq.Task) error {
	var p OrderDeclinedPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	if err := SendEmail(p.Email, p.Envelope.Subject, p.Envelope.Body); err != nil {
		return err
	}
	log.Printf("[notify] OrderDeclined sent -> order=%s to=%s", p.OrderID, p.Email)
	return nil
}

func handleOrderDelivered(_ context.Context, t *asynq.Task) error {
	var p OrderDeliveredPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	if err := SendEmail(p.Email, p.Envelope.Subject, p.Envelope.Body); err != nil {
		return err
	}
	log.Printf("[notify] OrderDelivered sent -> order=%s to=%s", p.OrderID, p.Email)
	return nil
}

func handleOrderCompleted(_ context.Context, t *asynq.Task) error {
	var p OrderCompletedPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	if err := SendEmail(p.Email, p.Envelope.Subject, p.Envelope.Body); err != nil {
		return err
	}
	log.Printf("[notify] OrderCompleted sent -> order=%s to=%s", p.OrderID, p.Email)
	return nil
}
