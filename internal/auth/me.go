package auth

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

// Me returns the currently authenticated user's profile
func Me(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid token claims"})
	}

	var id, name, email, role string
	err := db.Conn.QueryRow(context.Background(),
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
