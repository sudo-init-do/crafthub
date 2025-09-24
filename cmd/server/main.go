package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"github.com/sudo-init-do/crafthub/internal/auth"
	"github.com/sudo-init-do/crafthub/internal/db"
	custommw "github.com/sudo-init-do/crafthub/internal/middleware"
	"github.com/sudo-init-do/crafthub/internal/wallet"
)

func main() {
	// Load env file
	_ = godotenv.Load()

	// Connect DB
	db.Init()
	defer db.Conn.Close()

	// Init Echo
	e := echo.New()
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())

	// Health check
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "CraftHub API running")
	})

	// Auth routes (public)
	e.POST("/auth/signup", auth.Signup)
	e.POST("/auth/login", auth.Login)
	e.GET("/auth/me", auth.Me)

	// Wallet routes (protected with JWT middleware)
	walletGroup := e.Group("/wallet")
	walletGroup.Use(custommw.JWTMiddleware) 
	walletGroup.GET("/balance", wallet.Balance)
	walletGroup.POST("/topup/init", wallet.TopupInit)
	walletGroup.POST("/topup/confirm", wallet.ConfirmTopup)
	walletGroup.GET("/transactions", wallet.TransactionsHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("CraftHub API running on port %s", port)
	log.Fatal(e.Start(":" + port))
}
