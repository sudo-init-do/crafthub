package middleware

import (
    "net/http"

    "github.com/labstack/echo/v4"
)

// RequireRoles ensures the requester's role is one of the allowed roles.
// Usage: route(..., RequireRoles("admin"))
func RequireRoles(roles ...string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            role, _ := c.Get("role").(string)
            if role == "" {
                return c.JSON(http.StatusForbidden, echo.Map{"success": false, "error": "role missing"})
            }

            for _, r := range roles {
                if role == r {
                    return next(c)
                }
            }
            return c.JSON(http.StatusForbidden, echo.Map{"success": false, "error": "access denied"})
        }
    }
}

