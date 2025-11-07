package utils

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

var jwtSecret = []byte("supersecret") // Should match your JWT_SECRET env

// ExtractUserIDFromToken pulls the user ID from JWT in the Authorization header
func ExtractUserIDFromToken(c echo.Context) (string, error) {
	userToken := c.Request().Header.Get("Authorization")
	if userToken == "" {
		return "", errors.New("missing authorization header")
	}

	tokenStr := userToken[len("Bearer "):]
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if id, ok := claims["id"].(string); ok {
			return id, nil
		}
	}

	return "", errors.New("invalid token claims")
}
