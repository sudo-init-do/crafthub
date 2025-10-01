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
)

func main() {
	// Load env vars
	_ = godotenv.Load()

	// Init DB
	db.Init()
	defer db.Conn.Close()

	// Init Echo
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

	// Wallet
	walletGroup := protected.Group("/wallet")
	walletGroup.GET("/balance", wallet.Balance)
	walletGroup.POST("/topup/init", wallet.TopupInit)
	walletGroup.POST("/topup/confirm", wallet.ConfirmTopup)
	walletGroup.GET("/transactions", wallet.TransactionsHandler)
	walletGroup.POST("/withdraw/init", wallet.InitWithdrawal)
	walletGroup.POST("/withdraw/confirm", wallet.ConfirmWithdrawal)

	// ===== Admin Routes =====
	adminGroup := protected.Group("/admin")
	adminGroup.Use(custommw.AdminGuard)

	// Withdrawals Management
	adminGroup.GET("/withdrawals/pending", wallet.ListPendingWithdrawals)
	adminGroup.POST("/withdrawals/:id/approve", wallet.ApproveWithdrawal)
	adminGroup.POST("/withdrawals/:id/reject", wallet.RejectWithdrawal)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("CraftHub API running on port %s", port)
	log.Fatal(e.Start(":" + port))
}
