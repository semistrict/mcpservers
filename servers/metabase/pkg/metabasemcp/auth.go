package metabasemcp

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type Auther interface {
	Auth(ctx context.Context, r *http.Request) error
}

type CookieAuth struct {
	cookies map[string]string
}

func (c *CookieAuth) Auth(ctx context.Context, r *http.Request) error {
	for name, value := range c.cookies {
		r.AddCookie(&http.Cookie{
			Name:  name,
			Value: value,
		})
	}
	return nil
}

var _ Auther = &CookieAuth{}

// NewCookieAuth creates a new CookieAuth instance by reading cookies from a TSV file
func NewCookieAuth(cookiesFile string) (*CookieAuth, error) {
	file, err := os.Open(cookiesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open cookies file: %w", err)
	}
	defer file.Close()

	cookies := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Split by tab
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			// First column is name, second is value
			name := parts[0]
			value := parts[1]
			cookies[name] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading cookies file: %w", err)
	}

	return &CookieAuth{
		cookies: cookies,
	}, nil
}
