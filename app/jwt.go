package app

import (
	"errors"
	"time"

	"github.com/alpha-omega-corp/core/app/proto"
	jwt "github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthClaims struct {
	jwt.Claims
	User *proto.User
}

type AuthWrapper struct {
	secretKey string
	expiresAt int64
	provider  string
}

func NewAuthWrapper(key string) *AuthWrapper {

	return &AuthWrapper{
		secretKey: key,
		expiresAt: 24,
	}
}

func (w *AuthWrapper) GenerateToken(user *proto.User) (signedToken string, err error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &AuthClaims{
		User: user,
	})

	signedToken, err = token.SignedString([]byte(w.secretKey))

	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func (w *AuthWrapper) ValidateToken(signedToken string) (claims *AuthClaims, err error) {
	token, err := jwt.ParseWithClaims(
		signedToken,
		&AuthClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(w.secretKey), nil
		},
	)

	if err != nil {
		return
	}

	claims, ok := token.Claims.(*AuthClaims)

	if !ok {
		return nil, errors.New("unable to parse claims")
	}

	expiresAt, err := claims.GetExpirationTime()
	if err != nil {
		return nil, err
	}

	if expiresAt.Unix() < time.Now().Local().Unix() {
		return nil, errors.New("token is expired")
	}

	return claims, nil
}

func HashPassword(pw string) string {
	bytes, _ := bcrypt.GenerateFromPassword([]byte(pw), 5)

	return string(bytes)
}

func CheckPasswordHash(pw string, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw))

	return err == nil
}
