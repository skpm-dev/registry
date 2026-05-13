package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type User struct {
	Login string `json:"login"`
}

func GetAuthenticatedUser(token string) (*User, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach github: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid github token")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github returned %d", resp.StatusCode)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("could not decode github response: %w", err)
	}

	return &user, nil
}

func ExtractToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", fmt.Errorf("missing Authorization header")
	}

	token, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok {
		return "", fmt.Errorf("authorization header must be in format: Bearer <token>")
	}

	return token, nil
}
