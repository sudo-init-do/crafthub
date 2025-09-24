package auth

import (
	"context"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"github.com/sudo-init-do/crafthub/internal/db"
)

// Me returns the currently authenticated user's profile
func Me(c echo.Context) error {
	// Get Authorization header
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "missing Authorization header"})
	}

	const prefix = "Bearer "
	if len(authHeader) <= len(prefix) || authHeader[:len(prefix)] != prefix {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid Authorization format"})
	}
	tokenStr := authHeader[len(prefix):]

	// Parse JWT
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil || !token.Valid {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid or expired token"})
	}

	// Extract user_id from claims
	userID, ok := claims["user_id"].(string)
	if !ok {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid token claims"})
	}

	// Query DB for user
	var id, name, email, role string
	err = db.Conn.QueryRow(context.Background(),
		`SELECT id, name, email, role FROM users WHERE id=$1`, userID).
		Scan(&id, &name, &email, &role)

	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "user not found"})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"id":    id,
		"name":  name,
		"email": email,
		"role":  role,
	})
}
