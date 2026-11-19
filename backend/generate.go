// Package backend provides the reproducible API documentation generation command.
package backend

//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --generalInfo cmd/api/main.go --parseInternal --output docs --outputTypes go,json,yaml
