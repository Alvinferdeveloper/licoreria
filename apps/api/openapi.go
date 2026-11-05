// Package api holds the files embedded from the root of the API module.
package api

import _ "embed"

// OpenAPISpec is the REST contract, served at /api/v1/openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
