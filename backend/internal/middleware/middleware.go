package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/handler/api"
)

func JWTAuth(cfg *config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Support Authorization header, query param (?token=), or cookie
		tokenStr := ""
		if auth := c.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		} else if t := c.Query("token"); t != "" {
			tokenStr = t
		}
		if tokenStr == "" {
			return api.Unauthorized(c, "missing authorization header")
		}
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			return api.Unauthorized(c, "invalid token")
		}

		claims := token.Claims.(jwt.MapClaims)
		c.Locals("user_id", claims["sub"])
		return c.Next()
	}
}