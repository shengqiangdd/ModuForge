package main

import (
	"fmt"
	"time"

	"github.com/moduforge/backend/internal/service"
)

func main() {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1aWQiOiJlYzhmYjM1YS1hZjE2LTRlZjctYmU5MS02MWUzZGNiMmViNTMiLCJ1c2VybmFtZSI6IndzdGVzdDIiLCJyb2xlIjoidXNlciIsImlzcyI6Im1vZHVmb3JnZSIsImV4cCI6MTc4NTU1NjQ4MywiaWF0IjoxNzg0OTUxNjgzfQ.3utoFYirT2MYTA4zAu5JJiIJq29b3l0JjYDTnkbVFXI"
	secret := "change-me-in-production"

	// Test ParseJWT
	claims, err := service.ParseJWT(token, secret)
	if err != nil {
		fmt.Println("ParseJWT error:", err)
	} else {
		fmt.Println("ParseJWT OK:", claims.UID)
	}

	// Test ParseJWTAllowExpired
	claims2, err2 := service.ParseJWTAllowExpired(token, secret, 7*24*time.Hour)
	if err2 != nil {
		fmt.Println("ParseJWTAllowExpired error:", err2)
	} else {
		fmt.Println("ParseJWTAllowExpired OK:", claims2.UID)
	}
}
