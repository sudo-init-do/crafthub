package main

import (
    "log"
    "net/http"
    "os"

    "github.com/labstack/echo/v4"
    "github.com/labstack/echo/v4/middleware"

    appmw "github.com/sudo-init-do/crafthub/internal/middleware"
    "github.com/sudo-init-do/crafthub/internal/db"
    "github.com/sudo-init-do/crafthub/internal/alerts"
    // handlers
    auth "github.com/sudo-init-do/crafthub/internal/auth"
    market "github.com/sudo-init-do/crafthub/internal/marketplace"
    w "github.com/sudo-init-do/crafthub/internal/wallet"
    admin "github.com/sudo-init-do/crafthub/internal/admin"
    user "github.com/sudo-init-do/crafthub/internal/user"
)

func main() {
    // Init subsystems
    db.Init()
    alerts.Init()

    e := echo.New()
    e.HideBanner = true
    e.Use(middleware.Logger())
    e.Use(middleware.Recover())

    // Health
    e.GET("/health", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

    // Public auth routes
    e.POST("/signup", auth.Signup)
    e.POST("/login", auth.Login)
    e.GET("/user/:id/profile", user.GetPublicProfile)

    // Authenticated group
    g := e.Group("")
    g.Use(appmw.JWTMiddleware)

    // Me and profile update
    g.GET("/me", auth.Me)
    g.PATCH("/user/profile", user.UpdateProfile)

    // Wallet
    g.GET("/wallet/balance", w.Balance)
    g.POST("/wallet/topups/init", w.TopupInit)
    g.POST("/wallet/topups/:id/confirm", w.ConfirmTopup)
    g.POST("/wallet/withdraw/init", w.InitWithdrawal)
    g.POST("/wallet/withdraw/:id/confirm", w.ConfirmWithdrawal)
    g.GET("/wallet/transactions", w.GetUserTransactions)

    // Marketplace services
    g.POST("/marketplace/services", market.CreateService)
    e.GET("/marketplace/services", market.GetAllServices) // public discovery
    g.GET("/marketplace/services/me", market.GetUserServices)

    // Marketplace orders
    g.POST("/marketplace/orders", market.CreateOrder)
    g.POST("/marketplace/orders/:id/accept", market.AcceptOrder)
    g.POST("/marketplace/orders/:id/reject", market.RejectOrder)
    g.POST("/marketplace/orders/:id/confirm", market.ConfirmOrder)
    g.POST("/marketplace/orders/:id/cancel", market.CancelOrder)
    g.POST("/marketplace/orders/:id/decline", market.DeclineOrder)
    g.POST("/marketplace/orders/:id/deliver", market.DeliverOrder)
    g.POST("/marketplace/orders/:id/complete", market.CompleteOrder)
    g.GET("/marketplace/orders", market.GetUserOrders)
    g.POST("/admin/orders/:id/release", market.ReleaseOrder, appmw.AdminGuard)

    // Reviews
    g.POST("/marketplace/orders/:id/review", market.CreateReview)
    e.GET("/marketplace/sellers/:id/reviews", market.GetSellerReviews)
    g.GET("/marketplace/orders/:id/review", market.GetOrderReview)

    // Admin routes
    adminGroup := e.Group("/admin")
    adminGroup.Use(appmw.JWTMiddleware)
    adminGroup.Use(appmw.AdminGuard)
    adminGroup.GET("/stats", admin.Stats)
    adminGroup.GET("/bookings", admin.ListBookings)
    adminGroup.GET("/wallets", admin.ListWallets)
    adminGroup.GET("/transactions", w.AdminGetAllTransactions)
    adminGroup.GET("/users", admin.ListUsers)
    adminGroup.POST("/users/:id/suspend", admin.SuspendUser)
    adminGroup.POST("/users/:id/activate", admin.ActivateUser)
    adminGroup.POST("/users/:id/promote_creator", admin.PromoteCreator)
    adminGroup.POST("/users/:id/demote_creator", admin.DemoteCreator)

    port := os.Getenv("PORT")
    if port == "" { port = "8080" }
    log.Printf("API server listening on :%s", port)
    if err := e.Start(":" + port); err != nil {
        log.Fatalf("server error: %v", err)
    }
}

