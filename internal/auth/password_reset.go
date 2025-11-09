package auth

import (
    "context"
    "fmt"
    "net/http"
    "net/url"
    "os"
    "strings"
    "time"

    "github.com/golang-jwt/jwt/v4"
    "github.com/labstack/echo/v4"
    "golang.org/x/crypto/bcrypt"

    "github.com/sudo-init-do/crafthub/internal/alerts"
    "github.com/sudo-init-do/crafthub/internal/db"
)

type RequestPasswordResetRequest struct {
    Email string `json:"email" validate:"required,email"`
}

type RequestPasswordResetResponse struct {
    Message string `json:"message"`
}

// POST /auth/password/request
// Always responds with success message to avoid user enumeration.
func RequestPasswordReset(c echo.Context) error {
    req := new(RequestPasswordResetRequest)
    if err := c.Bind(req); err != nil || req.Email == "" {
        // Still return generic message
        return c.JSON(http.StatusOK, RequestPasswordResetResponse{Message: "If the email exists, a reset link has been sent."})
    }

    // Try to locate the user (id and name)
    var userID string
    var name string
    err := db.Conn.QueryRow(context.Background(), `SELECT id, name FROM users WHERE email = $1`, req.Email).Scan(&userID, &name)
    if err != nil || userID == "" {
        // Do not reveal existence
        return c.JSON(http.StatusOK, RequestPasswordResetResponse{Message: "If the email exists, a reset link has been sent."})
    }

    // Create a short-lived password reset token
    expiryMinutes := 30
    if v := os.Getenv("PASSWORD_RESET_EXP_MINUTES"); v != "" {
        // best-effort parse
        if dur, parseErr := time.ParseDuration(v + "m"); parseErr == nil {
            expiryMinutes = int(dur.Minutes())
        }
    }
    claims := jwt.MapClaims{
        "user_id": userID,
        "purpose": "password_reset",
        "exp":     time.Now().Add(time.Duration(expiryMinutes) * time.Minute).Unix(),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, signErr := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
    if signErr != nil {
        // Fail silently but inform client generically
        return c.JSON(http.StatusOK, RequestPasswordResetResponse{Message: "If the email exists, a reset link has been sent."})
    }

    // Build reset URL for frontend
    base := os.Getenv("APP_URL")
    if base == "" {
        base = "http://localhost:3000"
    }
    resetURL := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(base, "/"), url.QueryEscape(signed))

    // Enqueue email asynchronously (personalized)
    _ = alerts.EnqueuePasswordReset(userID, req.Email, resetURL, name)

    return c.JSON(http.StatusOK, RequestPasswordResetResponse{Message: "If the email exists, a reset link has been sent."})
}

type ResetPasswordRequest struct {
    Token       string `json:"token" validate:"required"`
    NewPassword string `json:"new_password" validate:"required,min=6"`
}

// POST /auth/password/reset
func ResetPassword(c echo.Context) error {
    req := new(ResetPasswordRequest)
    if err := c.Bind(req); err != nil || req.Token == "" || len(req.NewPassword) < 6 {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
    }

    // Validate token
    parsed, err := jwt.Parse(req.Token, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method")
        }
        return []byte(os.Getenv("JWT_SECRET")), nil
    })
    if err != nil || !parsed.Valid {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid or expired token"})
    }

    claims, ok := parsed.Claims.(jwt.MapClaims)
    if !ok {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid token claims"})
    }
    purpose, _ := claims["purpose"].(string)
    if purpose != "password_reset" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid token purpose"})
    }
    userID, _ := claims["user_id"].(string)
    if userID == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid token subject"})
    }

    // Hash new password
    hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "server error"})
    }

    // Update DB
    ct, err := db.Conn.Exec(context.Background(), `UPDATE users SET password = $1 WHERE id = $2`, string(hashed), userID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update password"})
    }
    if ct.RowsAffected() == 0 {
        return c.JSON(http.StatusNotFound, echo.Map{"error": "user not found"})
    }

    return c.JSON(http.StatusOK, echo.Map{"message": "password updated successfully"})
}
