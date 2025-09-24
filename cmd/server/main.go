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
	"github.com/sudo-init-do/crafthub/internal/middleware"
	"github.com/sudo-init-do/crafthub/internal/wallet"
)

func main() {
	// Load env file
	_ = godotenv.Load()

	// Connect DB
	dbConn := db.Connect()
	defer dbConn.Close()

	// Init Echo
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Health check
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "CraftHub API running")
	})

	// Auth routes (public)
	e.POST("/auth/signup", auth.Signup)
	e.POST("/auth/login", auth.Login)
	e.GET("/auth/me", auth.Me)

	// Wallet routes (protected with JWT)
	walletGroup := e.Group("/wallet", middlewarex.JWTMiddleware)
	walletGroup.GET("/balance", wallet.Balance)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("CraftHub API running on port %s", port)
	log.Fatal(e.Start(":" + port))
}
