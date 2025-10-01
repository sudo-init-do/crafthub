package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// AdminGuard ensures only admin users can access admin routes
func AdminGuard(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		role, ok := c.Get("role").(string)
		if !ok || role != "admin" {
			return c.JSON(http.StatusForbidden, echo.Map{
				"error": "admin access only",
			})
		}
		return next(c)
	}
}
