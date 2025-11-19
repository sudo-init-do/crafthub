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
        Title             string  `json:"title"`
        Description       string  `json:"description"`
        Price             float64 `json:"price"`
        Category          string  `json:"category"`
        DeliveryTimeDays  int     `json:"delivery_time_days"`
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
		`INSERT INTO services (id, user_id, title, description, price, category, delivery_time_days, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'active', $8)`,
		serviceID, uid, req.Title, req.Description, req.Price, req.Category, req.DeliveryTimeDays, time.Now(),
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
    category := c.QueryParam("category")
    deliveryMax := c.QueryParam("delivery_time_max")
    ratingMin := c.QueryParam("rating_min")
    sort := c.QueryParam("sort")
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
    // Aggregated query to support rating filter/sort
    query := `SELECT s.id, s.user_id, s.title, s.description, s.price, s.category, s.delivery_time_days, s.status, s.created_at,
                     COALESCE(AVG(r.rating)::float, 0) AS avg_rating
              FROM services s
              LEFT JOIN orders o ON o.service_id = s.id
              LEFT JOIN reviews r ON r.order_id = o.id`
    var where []string
    var args []any

    if q != "" {
        where = append(where, "(title ILIKE $%d OR description ILIKE $%d)")
        // We'll add the same arg twice for title and description
        qArg := "%" + q + "%"
        args = append(args, qArg, qArg)
    }
    if minPrice != "" {
        where = append(where, "s.price >= $%d")
        args = append(args, minPrice)
    }
    if maxPrice != "" {
        where = append(where, "s.price <= $%d")
        args = append(args, maxPrice)
    }
    if category != "" {
        where = append(where, "s.category = $%d")
        args = append(args, category)
    }
    if deliveryMax != "" {
        where = append(where, "s.delivery_time_days <= $%d")
        args = append(args, deliveryMax)
    }
    if ratingMin != "" {
        // We will apply HAVING after GROUP BY; keep placeholder for now
        // Use a marker to render later
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
        // Keep idx for later appends
        // idx reflects next parameter position
        // We will reuse it below
        // But since idx is local, recompute current parameter count
    }
    // Group by service fields
    query += " GROUP BY s.id ORDER BY "
    switch sort {
    case "price_asc":
        query += "s.price ASC"
    case "price_desc":
        query += "s.price DESC"
    case "rating_desc":
        query += "avg_rating DESC"
    case "oldest":
        query += "s.created_at ASC"
    default:
        query += "s.created_at DESC"
    }
    // Optional rating_min HAVING clause
    if ratingMin != "" {
        // Append HAVING with next parameter index
        currentIdx := 1
        for range args { currentIdx++ }
        query = strings.Replace(query, " GROUP BY s.id ORDER BY ", fmt.Sprintf(" GROUP BY s.id HAVING COALESCE(AVG(r.rating)::float,0) >= $%d ORDER BY ", currentIdx), 1)
        args = append(args, ratingMin)
    }
    // Append limit and offset with next indices
    currentIdx := 1
    for range args { currentIdx++ }
    query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", currentIdx, currentIdx+1)
    args = append(args, limit, offset)

    rows, err := db.Conn.Query(context.Background(), query, args...)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch services"})
    }
    defer rows.Close()

    var services []ServiceSummary
    for rows.Next() {
        var s ServiceSummary
        if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.Description, &s.Price, &s.Category, &s.DeliveryTimeDays, &s.Status, &s.CreatedAt, &s.AvgRating); err != nil {
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
