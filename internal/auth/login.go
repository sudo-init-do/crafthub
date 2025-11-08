package auth

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/sudo-init-do/crafthub/internal/db"
)

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

// ===== Login =====
func Login(c echo.Context) error {
	req := new(LoginRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	conn := db.Conn
	ctx := context.Background()

	var (
		userID   string
		password string
		role     string
	)
	err := conn.QueryRow(ctx, `
        SELECT id, password, role FROM users WHERE email = $1
    `, req.Email).Scan(&userID, &password, &role)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid credentials"})
	}
	// Determine is_active if the column exists; default TRUE if missing
	var isActive bool = true
	var hasActiveCol bool
	if err := conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'is_active'
        )
    `).Scan(&hasActiveCol); err == nil && hasActiveCol {
		_ = conn.QueryRow(ctx, `SELECT COALESCE(is_active, TRUE) FROM users WHERE id = $1`, userID).Scan(&isActive)
	}
	if !isActive {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "account suspended"})
	}
	if err := bcrypt.CompareHashAndPassword([]byte(password), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid credentials"})
	}

	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(72 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "token generation failed"})
	}

	return c.JSON(http.StatusOK, LoginResponse{Token: signed})
}
