package app

import (
	"github.com/rs/cors"
	"github.com/uptrace/bunrouter"
	"net/http"
)

type AuthMiddleware struct {
	authClient AuthClient
}

func NewAuthMiddleware(client AuthClient) *AuthMiddleware {
	return &AuthMiddleware{
		authClient: client,
	}
}

func (m *AuthMiddleware) Auth(next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
	return func(w http.ResponseWriter, req bunrouter.Request) error {
		err := m.authClient.Validate(w, req)
		if err != nil {
			return err
		}

		return next(w, req)
	}
}

func NewCorsMiddleware() bunrouter.MiddlewareFunc {
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:4000"},
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE"},
		AllowCredentials: true,
	})

	return func(next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
		return bunrouter.HTTPHandler(corsHandler.Handler(next))
	}
}
