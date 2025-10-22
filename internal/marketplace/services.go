package marketplace

import (
	"context"
	"net/http"
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
	rows, err := db.Conn.Query(
		context.Background(),
		`SELECT id, user_id, title, description, price, created_at
		 FROM services ORDER BY created_at DESC`,
	)
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
