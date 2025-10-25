package marketplace

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/sudo-init-do/crafthub/internal/db"
)

// CreateReview allows a buyer to rate and review a completed order
func CreateReview(c echo.Context) error {
	buyerID, ok := c.Get("user_id").(string)
	if !ok || buyerID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	orderID := c.Param("id")
	if orderID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id"})
	}

	// Validate UUID format
	if _, err := uuid.Parse(orderID); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid order id format"})
	}

	var req CreateReviewRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	// Validate rating
	if req.Rating < 1 || req.Rating > 5 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "rating must be between 1 and 5"})
	}

	// Validate comment length
	if len(req.Comment) > 1000 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "comment too long (max 1000 characters)"})
	}

	ctx := context.Background()

	log.Printf("DEBUG: Creating review for order %s by user %s", orderID, buyerID)

	// Check if order exists, is completed, and belongs to this buyer
	var sellerID string
	var orderStatus string
	orderErr := db.Conn.QueryRow(ctx,
		`SELECT seller_id, status FROM orders WHERE id = $1::uuid AND buyer_id = $2::uuid`,
		orderID, buyerID,
	).Scan(&sellerID, &orderStatus)
	if orderErr != nil {
		log.Printf("DEBUG: Order lookup failed: %v", orderErr)
		if errors.Is(orderErr, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, echo.Map{
				"error":    "order not found or not yours",
				"order_id": orderID,
				"user_id":  buyerID,
			})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error":   "failed to fetch order",
			"details": orderErr.Error(),
		})
	}

	log.Printf("DEBUG: Order found - seller: %s, status: %s", sellerID, orderStatus)

	// Only allow reviews for completed orders
	if orderStatus != "completed" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error":        "can only review completed orders",
			"order_status": orderStatus,
		})
	}

	log.Printf("DEBUG: Checking if review already exists for order %s", orderID)

	// Check if review already exists for this order
	var existingReviewID string
	reviewCheckErr := db.Conn.QueryRow(ctx,
		`SELECT id FROM reviews WHERE order_id = $1::uuid`,
		orderID,
	).Scan(&existingReviewID)

	if reviewCheckErr != nil {
		if errors.Is(reviewCheckErr, pgx.ErrNoRows) {
			// No review exists yet, this is expected and good
			log.Printf("DEBUG: No existing review found (as expected), proceeding to create new review")
		} else {
			// Some other database error occurred
			log.Printf("DEBUG: Review check failed with unexpected error: %v", reviewCheckErr)
			return c.JSON(http.StatusInternalServerError, echo.Map{
				"error":   "failed to check existing review",
				"details": reviewCheckErr.Error(),
			})
		}
	} else {
		// Review exists, don't allow duplicate
		log.Printf("DEBUG: Review already exists with ID: %s", existingReviewID)
		return c.JSON(http.StatusConflict, echo.Map{"error": "review already exists for this order"})
	}

	// Create the review
	reviewID := uuid.New().String()
	now := time.Now()

	log.Printf("DEBUG: Creating review with ID: %s", reviewID)

	_, insertErr := db.Conn.Exec(ctx,
		`INSERT INTO reviews (id, order_id, buyer_id, seller_id, rating, comment, created_at, updated_at)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7, $8)`,
		reviewID, orderID, buyerID, sellerID, req.Rating, req.Comment, now, now,
	)
	if insertErr != nil {
		log.Printf("DEBUG: Failed to insert review: %v", insertErr)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error":   "failed to create review",
			"details": insertErr.Error(),
		})
	}

	log.Printf("DEBUG: Review created successfully with ID: %s", reviewID)

	return c.JSON(http.StatusCreated, CreateReviewResponse{
		ReviewID: reviewID,
		Message:  "Review created successfully",
	})
}

// GetSellerReviews returns all reviews for a specific seller with rating summary
func GetSellerReviews(c echo.Context) error {
	sellerID := c.Param("id")
	if sellerID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing seller id"})
	}

	// Parse pagination parameters
	page := 1
	limit := 10
	if pageParam := c.QueryParam("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}
	if limitParam := c.QueryParam("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	offset := (page - 1) * limit
	ctx := context.Background()

	// Check if seller exists
	var sellerName string
	err := db.Conn.QueryRow(ctx,
		`SELECT name FROM users WHERE id = $1`,
		sellerID,
	).Scan(&sellerName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "seller not found"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch seller"})
	}

	// Get rating summary
	var summary SellerRatingSummary
	summary.SellerID = sellerID
	summary.SellerName = sellerName

	// Get total reviews and average rating
	err = db.Conn.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(AVG(rating), 0) FROM reviews WHERE seller_id = $1`,
		sellerID,
	).Scan(&summary.TotalReviews, &summary.AverageRating)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch rating summary"})
	}

	// Get rating counts breakdown
	rows, err := db.Conn.Query(ctx,
		`SELECT rating, COUNT(*) FROM reviews WHERE seller_id = $1 GROUP BY rating ORDER BY rating DESC`,
		sellerID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch rating breakdown"})
	}
	defer rows.Close()

	for rows.Next() {
		var rating, count int
		if err := rows.Scan(&rating, &count); err != nil {
			continue
		}
		switch rating {
		case 5:
			summary.RatingCounts.FiveStar = count
		case 4:
			summary.RatingCounts.FourStar = count
		case 3:
			summary.RatingCounts.ThreeStar = count
		case 2:
			summary.RatingCounts.TwoStar = count
		case 1:
			summary.RatingCounts.OneStar = count
		}
	}

	// Get reviews with buyer details
	reviewRows, err := db.Conn.Query(ctx,
		`SELECT r.id, r.order_id, r.buyer_id, u.name, r.seller_id, r.rating, r.comment, r.created_at, r.updated_at
		 FROM reviews r
		 JOIN users u ON r.buyer_id = u.id
		 WHERE r.seller_id = $1
		 ORDER BY r.created_at DESC
		 LIMIT $2 OFFSET $3`,
		sellerID, limit, offset,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch reviews"})
	}
	defer reviewRows.Close()

	var reviews []ReviewWithDetails
	for reviewRows.Next() {
		var review ReviewWithDetails
		if err := reviewRows.Scan(
			&review.ID, &review.OrderID, &review.BuyerID, &review.BuyerName,
			&review.SellerID, &review.Rating, &review.Comment,
			&review.CreatedAt, &review.UpdatedAt,
		); err != nil {
			continue
		}
		reviews = append(reviews, review)
	}

	return c.JSON(http.StatusOK, echo.Map{
		"seller_summary": summary,
		"reviews":        reviews,
		"pagination": echo.Map{
			"page":  page,
			"limit": limit,
			"total": summary.TotalReviews,
		},
	})
}

// GetOrderReview returns the review for a specific order (if it exists)
func GetOrderReview(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "unauthorized"})
	}

	orderID := c.Param("id")
	if orderID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing order id"})
	}

	ctx := context.Background()

	// Check if user is involved in this order (buyer or seller)
	var buyerID, sellerID string
	err := db.Conn.QueryRow(ctx,
		`SELECT buyer_id, seller_id FROM orders WHERE id = $1`,
		orderID,
	).Scan(&buyerID, &sellerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "order not found"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch order"})
	}

	if userID != buyerID && userID != sellerID {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "not authorized to view this order's review"})
	}

	// Get the review if it exists
	var review ReviewWithDetails
	err = db.Conn.QueryRow(ctx,
		`SELECT r.id, r.order_id, r.buyer_id, u.name, r.seller_id, r.rating, r.comment, r.created_at, r.updated_at
		 FROM reviews r
		 JOIN users u ON r.buyer_id = u.id
		 WHERE r.order_id = $1`,
		orderID,
	).Scan(
		&review.ID, &review.OrderID, &review.BuyerID, &review.BuyerName,
		&review.SellerID, &review.Rating, &review.Comment,
		&review.CreatedAt, &review.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "no review found for this order"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch review"})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"review": review,
	})
}
