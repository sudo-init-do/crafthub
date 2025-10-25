package marketplace

import "time"

// Review represents a rating and review given by a buyer for a completed order
type Review struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"order_id"`
	BuyerID   string    `json:"buyer_id"`
	SellerID  string    `json:"seller_id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReviewWithDetails represents a review with additional buyer information
type ReviewWithDetails struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"order_id"`
	BuyerID   string    `json:"buyer_id"`
	BuyerName string    `json:"buyer_name"`
	SellerID  string    `json:"seller_id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SellerRatingSummary represents aggregated rating data for a seller
type SellerRatingSummary struct {
	SellerID      string  `json:"seller_id"`
	SellerName    string  `json:"seller_name"`
	TotalReviews  int     `json:"total_reviews"`
	AverageRating float64 `json:"average_rating"`
	RatingCounts  struct {
		FiveStar  int `json:"five_star"`
		FourStar  int `json:"four_star"`
		ThreeStar int `json:"three_star"`
		TwoStar   int `json:"two_star"`
		OneStar   int `json:"one_star"`
	} `json:"rating_counts"`
}

// CreateReviewRequest represents the request payload for creating a review
type CreateReviewRequest struct {
	Rating  int    `json:"rating" validate:"required,min=1,max=5"`
	Comment string `json:"comment" validate:"max=1000"`
}

// CreateReviewResponse represents the response after creating a review
type CreateReviewResponse struct {
	ReviewID string `json:"review_id"`
	Message  string `json:"message"`
}
