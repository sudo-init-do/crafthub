package auth

import (
    "context"
    "net/http"
    "os"

    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

type BootstrapAdminRequest struct {
    Email  string `json:"email"`
    Secret string `json:"secret"`
}

func BootstrapAdmin(c echo.Context) error {
    req := new(BootstrapAdminRequest)
    if err := c.Bind(req); err != nil {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
    }

    cfgSecret := os.Getenv("ADMIN_BOOTSTRAP_SECRET")
    if cfgSecret == "" {
        return c.JSON(http.StatusForbidden, echo.Map{"error": "bootstrap disabled"})
    }
    if req.Secret == "" || req.Secret != cfgSecret {
        return c.JSON(http.StatusForbidden, echo.Map{"error": "invalid secret"})
    }
    if req.Email == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "email required"})
    }

    ct, err := db.Conn.Exec(context.Background(), `UPDATE users SET role = 'admin' WHERE email = $1`, req.Email)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to promote user"})
    }
    if ct.RowsAffected() == 0 {
        return c.JSON(http.StatusNotFound, echo.Map{"error": "user not found"})
    }
    return c.JSON(http.StatusOK, echo.Map{"message": "user promoted to admin", "email": req.Email})
}

