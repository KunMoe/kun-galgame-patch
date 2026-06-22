//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -config oapi-codegen.yaml openapi-3.0.yaml

// Package gen holds the artifact-service client generated from the committed
// OpenAPI contract. Do not edit artifact_client.gen.go by hand; re-run go generate.
package gen
