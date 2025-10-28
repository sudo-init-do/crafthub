package user

import (
	"database/sql"
	"net/http"

	"github.com/labstack/echo/v4"
)

var DB *sql.DB // We'll set this globally from db.Conn later

type UpdateProfileRequest struct {
	Name      string `json:"name"`
	Bio       string `json:"bio"`
	AvatarURL string `json:"avatar_url"`
}

func UpdateProfile(c echo.Context) error {
	userID := c.Get("user_id").(string) // depends on your JWT middleware
	if userID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	req := new(UpdateProfileRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	query := `
		UPDATE users
		SET 
			name = COALESCE(NULLIF($1, ''), name),
			bio = COALESCE(NULLIF($2, ''), bio),
			avatar_url = COALESCE(NULLIF($3, ''), avatar_url),
			updated_at = NOW()
		WHERE id = $4
	`
	_, err := DB.Exec(query, req.Name, req.Bio, req.AvatarURL, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update profile"})
	}

	return c.JSON(http.StatusOK, echo.Map{"message": "Profile updated successfully"})
}
