package user

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

// GET /user/:id/profile
func GetPublicProfile(c echo.Context) error {
	userID := c.Param("id")
	if userID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing user id"})
	}

	// Local struct for scanning
	var (
		id        string
		name      string
		bio       string
		avatarURL string
		role      string
		createdAt time.Time
	)

	query := `
		SELECT id, name, bio, avatar_url, role, created_at
		FROM users
		WHERE id = $1
	`

	err := db.Conn.QueryRow(context.Background(), query, userID).Scan(
		&id,
		&name,
		&bio,
		&avatarURL,
		&role,
		&createdAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "user not found"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error":   "failed to fetch user",
			"details": err.Error(),
		})
	}

	// Response payload
	profile := echo.Map{
		"id":         id,
		"name":       name,
		"bio":        bio,
		"avatar_url": avatarURL,
		"role":       role,
		"created_at": createdAt.Format(time.RFC3339),
	}

	return c.JSON(http.StatusOK, profile)
}
