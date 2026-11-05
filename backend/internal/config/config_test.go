package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	base := map[string]string{
		"DATABASE_URL": "postgres://user:pass@db:5432/uptime", "JWT_SECRET_BASE64": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("s", 32))),
		"JWT_ISSUER": "api", "JWT_AUDIENCE": "web", "ALLOWED_ORIGINS": "https://example.com",
	}
	for _, tc := range []struct{ key, value string }{
		{"DATABASE_URL", ""}, {"DATABASE_URL", "mysql://db/test"}, {"DATABASE_URL", "postgres://db/"},
		{"JWT_SECRET_BASE64", "invalid!"}, {"JWT_SECRET_BASE64", base64.StdEncoding.EncodeToString([]byte("short"))},
		{"JWT_ISSUER", " "}, {"JWT_AUDIENCE", ""}, {"HTTP_ADDR", ":0"}, {"HTTP_ADDR", "localhost"},
		{"LOCAL_HTTP", "maybe"}, {"ALLOWED_ORIGINS", "*"}, {"ALLOWED_ORIGINS", "null"},
		{"ALLOWED_ORIGINS", "https://example.com/path"}, {"ALLOWED_ORIGINS", "https://user@example.com"},
		{"ALLOWED_ORIGINS", "http://localhost:3000"}, {"ALLOWED_ORIGINS", ""},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			_, err := load(func(key string) string {
				if key == tc.key {
					return tc.value
				}
				return base[key]
			})
			if err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	cfg, err := load(func(key string) string { return base[key] })
	if err != nil || cfg.LocalHTTP || cfg.HTTPAddr != ":8080" || len(cfg.JWTSecret) != 32 {
		t.Fatalf("secure defaults: %+v, %v", cfg, err)
	}
	base["LOCAL_HTTP"], base["ALLOWED_ORIGINS"] = "true", "http://localhost:3000, https://example.com"
	if cfg, err := load(func(key string) string { return base[key] }); err != nil || !cfg.LocalHTTP || len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("explicit local HTTP: %+v, %v", cfg, err)
	}
}
