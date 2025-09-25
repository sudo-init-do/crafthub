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
	_ = godotenv.Load()

	db.Init()
	defer db.Conn.Close()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "CraftHub API running")
	})

	// Public auth
	e.POST("/auth/signup", auth.Signup)
	e.POST("/auth/login", auth.Login)

	// Protected routes
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("CraftHub API running on port %s", port)
	log.Fatal(e.Start(":" + port))
}
