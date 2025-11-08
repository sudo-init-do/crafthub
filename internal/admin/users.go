package admin

import (
    "context"
    "net/http"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

type AdminUser struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Role      string    `json:"role"`
    IsActive  bool      `json:"is_active"`
    CreatedAt time.Time `json:"created_at"`
}

// GET /admin/users
func ListUsers(c echo.Context) error {
    ctx := context.Background()

    var hasActiveColumn bool
    if err := db.Conn.QueryRow(ctx,
        `SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'is_active'
        )`).Scan(&hasActiveColumn); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not inspect schema"})
    }

    query := `SELECT id, name, email, role, TRUE as is_active, created_at FROM users ORDER BY created_at DESC`
    if hasActiveColumn {
        query = `SELECT id, name, email, role, COALESCE(is_active, TRUE) as is_active, created_at FROM users ORDER BY created_at DESC`
    }

    rows, err := db.Conn.Query(ctx, query)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch users"})
    }
    defer rows.Close()

    var users []AdminUser
    for rows.Next() {
        var u AdminUser
        if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.IsActive, &u.CreatedAt); err != nil {
            return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to read user record"})
        }
        users = append(users, u)
    }
    return c.JSON(http.StatusOK, echo.Map{"users": users})
}

// POST /admin/users/:id/suspend
func SuspendUser(c echo.Context) error {
    userID := c.Param("id")
    if userID == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "user id required"})
    }
    // Only attempt update if column exists
    var hasActiveColumn bool
    if err := db.Conn.QueryRow(context.Background(),
        `SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'is_active'
        )`).Scan(&hasActiveColumn); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not inspect schema"})
    }
    if !hasActiveColumn {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "user suspension unavailable: is_active column missing"})
    }
    _, err := db.Conn.Exec(context.Background(), `UPDATE users SET is_active = FALSE WHERE id = $1`, userID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to suspend user"})
    }
    return c.JSON(http.StatusOK, echo.Map{"message": "user suspended", "user_id": userID})
}

// POST /admin/users/:id/activate
func ActivateUser(c echo.Context) error {
    userID := c.Param("id")
    if userID == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "user id required"})
    }
    // Only attempt update if column exists
    var hasActiveColumn bool
    if err := db.Conn.QueryRow(context.Background(),
        `SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'is_active'
        )`).Scan(&hasActiveColumn); err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not inspect schema"})
    }
    if !hasActiveColumn {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "user activation unavailable: is_active column missing"})
    }
    _, err := db.Conn.Exec(context.Background(), `UPDATE users SET is_active = TRUE WHERE id = $1`, userID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to activate user"})
    }
    return c.JSON(http.StatusOK, echo.Map{"message": "user activated", "user_id": userID})
}

// POST /admin/users/:id/promote_creator
func PromoteCreator(c echo.Context) error {
    userID := c.Param("id")
    if userID == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "user id required"})
    }

    _, err := db.Conn.Exec(context.Background(), `UPDATE users SET role = 'creator' WHERE id = $1`, userID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to promote user"})
    }
    return c.JSON(http.StatusOK, echo.Map{"message": "user promoted to creator", "user_id": userID})
}

// POST /admin/users/:id/demote_creator
func DemoteCreator(c echo.Context) error {
    userID := c.Param("id")
    if userID == "" {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "user id required"})
    }

    _, err := db.Conn.Exec(context.Background(), `UPDATE users SET role = 'fan' WHERE id = $1`, userID)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to demote user"})
    }
    return c.JSON(http.StatusOK, echo.Map{"message": "user demoted to fan", "user_id": userID})
}
