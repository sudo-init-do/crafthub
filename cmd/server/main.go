package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/sudo-init-do/crafthub/internal/auth"
	"github.com/sudo-init-do/crafthub/internal/db"
	"github.com/sudo-init-do/crafthub/internal/marketplace"
	mware "github.com/sudo-init-do/crafthub/internal/middleware"
	"github.com/sudo-init-do/crafthub/internal/user"
	"github.com/sudo-init-do/crafthub/internal/wallet"
)

func main() {
	// Initialize database connection
	db.Init()

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

	api.POST("/marketplace/services", marketplace.CreateService, mware.RequireRoles("seller"))
	api.GET("/marketplace/services/me", marketplace.GetUserServices, mware.RequireRoles("seller"))

	api.POST("/marketplace/orders", marketplace.CreateOrder, mware.RequireRoles("fan"))
	api.POST("/marketplace/orders/:id/accept", marketplace.AcceptOrder, mware.RequireRoles("seller"))
	api.POST("/marketplace/orders/:id/confirm", marketplace.ConfirmOrder, mware.RequireRoles("seller"))
	// Release is an admin operation; wired under /admin below
	api.GET("/marketplace/orders/me", marketplace.GetUserOrders)
	api.POST("/marketplace/orders/:id/review", marketplace.CreateReview)
	api.GET("/marketplace/orders/:id/review", marketplace.GetOrderReview)

	// Admin routes
	admin := e.Group("/admin")
	admin.Use(mware.JWTMiddleware)
	admin.Use(mware.AdminGuard)

	admin.GET("/transactions", wallet.AdminGetAllTransactions)
	admin.GET("/transactions/user/:id", wallet.AdminGetUserTransactions)
	admin.GET("/topups/pending", wallet.ListPendingTopups)
	admin.POST("/orders/:id/release", marketplace.ReleaseOrder)
	admin.GET("/transactions/all", wallet.GetAllTransactions)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
