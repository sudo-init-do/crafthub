package marketplace

import (
    "context"
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "github.com/sudo-init-do/crafthub/internal/db"
)

// CreateService allows a user to list a new service on the marketplace
func CreateService(c echo.Context) error {
    uid, ok := c.Get("user_id").(string)
    if !ok || uid == "" {
        return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
    }
    role, _ := c.Get("role").(string)

	var req struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Price       float64 `json:"price"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

    if req.Title == "" || req.Price <= 0 {
        return c.JSON(http.StatusBadRequest, echo.Map{"error": "title and valid price are required"})
    }

    // Enforce role-based listing limits
    // Fans: up to 3 services; Creators: up to 50 services
    var serviceCount int
    if err := db.Conn.QueryRow(context.Background(),
        `SELECT COUNT(*) FROM services WHERE user_id = $1`, uid,
    ).Scan(&serviceCount); err == nil {
        var maxAllowed int = 3
        if role == "creator" {
            maxAllowed = 50
        }
        if serviceCount >= maxAllowed {
            return c.JSON(http.StatusForbidden, echo.Map{
                "error":   "listing limit reached",
                "role":    role,
                "max":     maxAllowed,
                "current": serviceCount,
            })
        }
    }

	serviceID := uuid.New().String()

	_, err := db.Conn.Exec(
		context.Background(),
		`INSERT INTO services (id, user_id, title, description, price, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		serviceID, uid, req.Title, req.Description, req.Price, time.Now(),
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create service"})
	}

	return c.JSON(http.StatusCreated, echo.Map{
		"service_id": serviceID,
		"message":    "service created successfully",
	})
}

// GetAllServices returns all services visible in the marketplace
func GetAllServices(c echo.Context) error {
    // Optional search and pagination
    q := c.QueryParam("q")
    minPrice := c.QueryParam("min_price")
    maxPrice := c.QueryParam("max_price")
    limit := 20
    offset := 0
    if l := c.QueryParam("limit"); l != "" {
        if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
            limit = v
        }
    }
    if o := c.QueryParam("offset"); o != "" {
        if v, err := strconv.Atoi(o); err == nil && v >= 0 {
            offset = v
        }
    }

    // Build dynamic conditions
    query := `SELECT id, user_id, title, description, price, created_at FROM services`
    var where []string
    var args []any

    if q != "" {
        where = append(where, "(title ILIKE $%d OR description ILIKE $%d)")
        // We'll add the same arg twice for title and description
        qArg := "%" + q + "%"
        args = append(args, qArg, qArg)
    }
    if minPrice != "" {
        where = append(where, "price >= $%d")
        args = append(args, minPrice)
    }
    if maxPrice != "" {
        where = append(where, "price <= $%d")
        args = append(args, maxPrice)
    }

    // Replace placeholders with correct positions
    // Build WHERE with correct $n index expansion
    if len(where) > 0 {
        // We need to render the %d with actual indices
        // Create a rendered slice
        rendered := make([]string, len(where))
        idx := 1
        for i, w := range where {
            // count occurrences of %d in w
            // For simplicity, handle up to two %d per condition (title/description)
            if strings.Count(w, "%d") == 2 {
                rendered[i] = fmt.Sprintf(w, idx, idx+1)
                idx += 2
            } else {
                rendered[i] = fmt.Sprintf(w, idx)
                idx++
            }
        }
        query += " WHERE " + strings.Join(rendered, " AND ")
    }
    query += " ORDER BY created_at DESC LIMIT $%d OFFSET $%d"
    // Append limit and offset as final args
    // Compute current arg index
    currentIdx := 1
    for _, a := range args {
        _ = a
        currentIdx++
    }
    query = fmt.Sprintf(query, currentIdx, currentIdx+1)
    args = append(args, limit, offset)

    rows, err := db.Conn.Query(context.Background(), query, args...)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch services"})
    }
    defer rows.Close()

	var services []Service
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.Description, &s.Price, &s.CreatedAt); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to parse service record"})
		}
		services = append(services, s)
	}

    return c.JSON(http.StatusOK, echo.Map{"services": services})
}

// GetUserServices returns all services created by the authenticated user
func GetUserServices(c echo.Context) error {
	uid, ok := c.Get("user_id").(string)
	if !ok || uid == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	rows, err := db.Conn.Query(
		context.Background(),
		`SELECT id, user_id, title, description, price, created_at
		 FROM services WHERE user_id = $1 ORDER BY created_at DESC`,
		uid,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch user services"})
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.Description, &s.Price, &s.CreatedAt); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to parse service record"})
		}
		services = append(services, s)
	}

	return c.JSON(http.StatusOK, echo.Map{"services": services})
}
