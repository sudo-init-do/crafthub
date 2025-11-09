package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/sudo-init-do/crafthub/internal/admin"
	"github.com/sudo-init-do/crafthub/internal/alerts"
	"github.com/sudo-init-do/crafthub/internal/auth"
	"github.com/sudo-init-do/crafthub/internal/db"
	"github.com/sudo-init-do/crafthub/internal/marketplace"
	mware "github.com/sudo-init-do/crafthub/internal/middleware"
	"github.com/sudo-init-do/crafthub/internal/user"
	"github.com/sudo-init-do/crafthub/internal/wallet"
)

func main() {
	// Load environment variables from .env if present
	_ = godotenv.Load()
	// Initialize database connection
	db.Init()

	// Initialize async job processor (notifications)
	alerts.Init()

	// Configure SMTP mailer from environment
	if err := alerts.ConfigureMailerFromEnv(); err != nil {
		log.Printf("SMTP not configured: %v", err)
	}

	e := echo.New()

	// Basic middleware
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	// Health and root routes
	e.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{"status": "ok", "service": "crafthub"})
	})
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
	})
	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
	})
	e.GET("/ready", func(c echo.Context) error {
		if db.Conn == nil {
			return c.JSON(http.StatusServiceUnavailable, echo.Map{"status": "not_ready", "error": "db not initialized"})
		}
		if err := db.Conn.Ping(context.Background()); err != nil {
			return c.JSON(http.StatusServiceUnavailable, echo.Map{"status": "not_ready", "error": "db unreachable"})
		}
		return c.JSON(http.StatusOK, echo.Map{"status": "ready"})
	})

	// Public routes
	// Auth routes with per-IP rate limiting to protect signup/login from abuse
	authGroup := e.Group("/auth")
	authGroup.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(20)))
	authGroup.POST("/signup", auth.Signup)
	authGroup.POST("/login", auth.Login)
	authGroup.POST("/bootstrap/admin", auth.BootstrapAdmin)
	// Password reset (public)
	authGroup.POST("/password/request", auth.RequestPasswordReset)
	authGroup.POST("/password/reset", auth.ResetPassword)

	e.GET("/user/:id/profile", user.GetPublicProfile)

	e.GET("/marketplace/services", marketplace.GetAllServices)
	e.GET("/sellers/:id/reviews", marketplace.GetSellerReviews)

	// Protected routes
	api := e.Group("")
	api.Use(mware.JWTMiddleware)

	api.GET("/auth/me", auth.Me)

	api.PATCH("/user/profile", user.UpdateProfile)

	api.GET("/wallet/balance", wallet.Balance)
	api.GET("/wallet/transactions", wallet.GetUserTransactions)
	api.POST("/wallet/topup", wallet.TopupInit)
	api.POST("/wallet/topup/confirm", wallet.ConfirmTopup)
	api.POST("/wallet/withdraw", wallet.InitWithdrawal)
	api.POST("/wallet/withdraw/confirm", wallet.ConfirmWithdrawal)

	// Allow both creators and fans to list and manage their services
	api.POST("/marketplace/services", marketplace.CreateService, mware.RequireRoles("creator", "fan"))
	api.GET("/marketplace/services/me", marketplace.GetUserServices, mware.RequireRoles("creator", "fan"))

	api.POST("/marketplace/orders", marketplace.CreateOrder, mware.RequireRoles("fan"))
	// Allow both creators and fans to act as sellers
	api.POST("/marketplace/orders/:id/accept", marketplace.AcceptOrder, mware.RequireRoles("creator", "fan"))
	api.POST("/marketplace/orders/:id/confirm", marketplace.ConfirmOrder, mware.RequireRoles("creator", "fan"))
	// Release is an admin operation; wired under /admin below
	api.GET("/marketplace/orders/me", marketplace.GetUserOrders)
	api.POST("/marketplace/orders/:id/review", marketplace.CreateReview)
	api.GET("/marketplace/orders/:id/review", marketplace.GetOrderReview)

	// Admin routes
	adminGroup := e.Group("/admin")
	adminGroup.Use(mware.JWTMiddleware)
	adminGroup.Use(mware.AdminGuard)

	// Admin panels / CRUD
	adminGroup.GET("/users", admin.ListUsers)
	adminGroup.POST("/users/:id/suspend", admin.SuspendUser)
	adminGroup.POST("/users/:id/activate", admin.ActivateUser)
	adminGroup.POST("/users/:id/promote_creator", admin.PromoteCreator)
	adminGroup.POST("/users/:id/demote_creator", admin.DemoteCreator)

	adminGroup.GET("/bookings", admin.ListBookings)
	adminGroup.GET("/wallets", admin.ListWallets)

	// Transactions
	adminGroup.GET("/transactions", wallet.AdminGetAllTransactions)
	adminGroup.GET("/transactions/user/:id", wallet.AdminGetUserTransactions)
	adminGroup.GET("/transactions/all", wallet.GetAllTransactions)

	// Topups
	adminGroup.GET("/topups/pending", wallet.ListPendingTopups)

	// Order release (admin action)
	adminGroup.POST("/orders/:id/release", marketplace.ReleaseOrder)

	// Stats
	adminGroup.GET("/stats", admin.Stats)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
