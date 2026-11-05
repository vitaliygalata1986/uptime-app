package config

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	JWTSecret      []byte
	JWTIssuer      string
	JWTAudience    string
	AllowedOrigins []string
	LocalHTTP      bool
}

func Load() (Config, error) { return load(os.Getenv) }

func load(get func(string) string) (Config, error) {
	c := Config{
		HTTPAddr: get("HTTP_ADDR"), DatabaseURL: get("DATABASE_URL"),
		JWTIssuer: get("JWT_ISSUER"), JWTAudience: get("JWT_AUDIENCE"),
	}
	if c.HTTPAddr == "" {
		c.HTTPAddr = ":8080"
	}
	_, port, err := net.SplitHostPort(c.HTTPAddr)
	n, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || n < 1 || n > 65535 {
		return c, fmt.Errorf("HTTP_ADDR must contain a valid host:port")
	}
	db, err := url.Parse(c.DatabaseURL)
	if err != nil || db.Host == "" || (db.Scheme != "postgres" && db.Scheme != "postgresql") || db.Path == "" || db.Path == "/" {
		return c, fmt.Errorf("DATABASE_URL must be a PostgreSQL connection URL with database name")
	}
	c.JWTSecret, err = base64.StdEncoding.Strict().DecodeString(get("JWT_SECRET_BASE64"))
	if err != nil || len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("JWT_SECRET_BASE64 must encode at least 32 random bytes")
	}
	if strings.TrimSpace(c.JWTIssuer) == "" || strings.TrimSpace(c.JWTAudience) == "" {
		return c, fmt.Errorf("JWT_ISSUER and JWT_AUDIENCE are required")
	}
	if v := get("LOCAL_HTTP"); v != "" {
		c.LocalHTTP, err = strconv.ParseBool(v)
		if err != nil {
			return c, fmt.Errorf("LOCAL_HTTP must be a boolean")
		}
	}
	for _, value := range strings.Split(get("ALLOWED_ORIGINS"), ",") {
		origin := strings.TrimSpace(value)
		u, err := url.Parse(origin)
		if err != nil || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" ||
			(u.Scheme != "https" && !(c.LocalHTTP && u.Scheme == "http")) || strings.Contains(origin, "*") {
			return c, fmt.Errorf("ALLOWED_ORIGINS must contain exact HTTP(S) origins; HTTP requires LOCAL_HTTP=true")
		}
		c.AllowedOrigins = append(c.AllowedOrigins, origin)
	}
	return c, nil
}
