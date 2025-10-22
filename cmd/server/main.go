package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/sudo-init-do/crafthub/internal/auth"
	"github.com/sudo-init-do/crafthub/internal/db"
	custommw "github.com/sudo-init-do/crafthub/internal/middleware"
	"github.com/sudo-init-do/crafthub/internal/wallet"
	"github.com/sudo-init-do/crafthub/internal/marketplace"
)

func main() {
	// Load environment variables
	_ = godotenv.Load()

	// Initialize DB
	db.Init()
	defer db.Conn.Close()

	// Initialize Echo
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Health check
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "CraftHub API running")
	})

	// ===== Public Auth Routes =====
	e.POST("/auth/signup", auth.Signup)
	e.POST("/auth/login", auth.Login)

	// ===== Protected Routes =====
	protected := e.Group("")
	protected.Use(custommw.JWTMiddleware)

	// Auth
	protected.GET("/auth/me", auth.Me)

	// ===== Wallet Routes =====
	walletGroup := protected.Group("/wallet")
	walletGroup.GET("/balance", wallet.Balance)
	walletGroup.POST("/topup/init", wallet.TopupInit)
	walletGroup.POST("/topup/confirm", wallet.ConfirmTopup)
	walletGroup.GET("/transactions", wallet.GetUserTransactions)
	walletGroup.POST("/withdraw/init", wallet.InitWithdrawal)
	walletGroup.POST("/withdraw/confirm", wallet.ConfirmWithdrawal)

	// ===== Admin Routes =====
	adminGroup := protected.Group("/admin")
	adminGroup.Use(custommw.AdminGuard)
	adminGroup.GET("/topups/pending", wallet.ListPendingTopups)
	adminGroup.POST("/topup/confirm", wallet.ConfirmTopup)
	adminGroup.GET("/transactions", wallet.AdminGetAllTransactions)
	adminGroup.GET("/user/:id/transactions", wallet.AdminGetUserTransactions)

	// ===== Marketplace Routes =====
	marketGroup := protected.Group("/marketplace")

	// Services
	marketGroup.POST("/services", marketplace.CreateService)
	marketGroup.GET("/services", marketplace.GetAllServices)
	marketGroup.GET("/my/services", marketplace.GetUserServices)

	// Orders
	marketGroup.POST("/orders", marketplace.CreateOrder)
	marketGroup.GET("/orders", marketplace.GetUserOrders)

	// Seller confirms order (holds buyer funds in escrow)
	marketGroup.POST("/orders/:id/confirm", marketplace.ConfirmOrder)

	// Buyer marks order as complete (releases funds to seller)
	marketGroup.POST("/orders/:id/complete", marketplace.CompleteOrder)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("CraftHub API running on port %s", port)
	log.Fatal(e.Start(":" + port))
}
