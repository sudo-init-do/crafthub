package user

import (
    "net/http"

    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

type UpdateProfileRequest struct {
	Name      string `json:"name"`
	Bio       string `json:"bio"`
	AvatarURL string `json:"avatar_url"`
}

// PATCH /user/profile
func UpdateProfile(c echo.Context) error {
    userIDVal := c.Get("user_id")
    userID, ok := userIDVal.(string)
    if !ok || userID == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid or missing token"})
    }

	var req UpdateProfileRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	query := `
		UPDATE users 
		SET name = COALESCE(NULLIF($1, ''), name),
		    bio = COALESCE(NULLIF($2, ''), bio),
		    avatar_url = COALESCE(NULLIF($3, ''), avatar_url)
		WHERE id = $4
	`
    _, err := db.Conn.Exec(c.Request().Context(), query, req.Name, req.Bio, req.AvatarURL, userID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to update profile"})
    }

	return c.JSON(http.StatusOK, echo.Map{
		"message": "profile updated successfully",
	})
}
