package auth

import (
    "context"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/golang-jwt/jwt/v4"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/labstack/echo/v4"
    "golang.org/x/crypto/bcrypt"

    "github.com/sudo-init-do/crafthub/internal/db"
)

type SignupRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type SignupResponse struct {
	Token string `json:"token"`
}

// ===== Signup =====
func Signup(c echo.Context) error {
    req := new(SignupRequest)
    if err := c.Bind(req); err != nil {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
    }

    // Basic validation
    if strings.TrimSpace(req.Name) == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "name is required"})
    }
    if strings.TrimSpace(req.Email) == "" || !strings.Contains(req.Email, "@") {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "valid email is required"})
    }
    if len(req.Password) < 6 {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "password must be at least 6 characters"})
    }

	// Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "server error"})
	}

    conn := db.Conn
    ctx := context.Background()

    tx, err := conn.Begin(ctx)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "db transaction error"})
    }
    defer tx.Rollback(ctx)

    // Check email uniqueness explicitly
    var existingID string
    err = conn.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, req.Email).Scan(&existingID)
    if err == nil && existingID != "" {
        return c.JSON(http.StatusConflict, echo.Map{"error": "email already exists"})
    }
    if err != nil && err != pgx.ErrNoRows {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to check email uniqueness"})
    }

	// Default role is always "fan"
	var userID string
    err = tx.QueryRow(ctx, `
        INSERT INTO users (id, name, email, password, role)
        VALUES ($1, $2, $3, $4, 'fan')
        RETURNING id
    `, uuid.New().String(), req.Name, req.Email, string(hashed)).Scan(&userID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to create user"})
    }

	// Create wallet for user
	_, err = tx.Exec(ctx, `
		INSERT INTO wallets (user_id, balance, created_at)
		VALUES ($1, 0, $2)
	`, userID, time.Now())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "wallet creation failed"})
	}

	if err := tx.Commit(ctx); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "transaction failed"})
	}

	// JWT with user_id
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    "fan",
		"exp":     time.Now().Add(72 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "token generation failed"})
	}

	return c.JSON(http.StatusOK, SignupResponse{Token: signed})
}
