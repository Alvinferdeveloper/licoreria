// Package apidocs serves the OpenAPI contract and a Swagger UI page to try it.
package apidocs

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	api "licoreria/api"
)

const (
	swaggerBase = "https://unpkg.com/swagger-ui-dist@5.17.14"
	initScript  = `window.ui = SwaggerUIBundle({ url: "openapi.yaml", dom_id: "#ui", deepLinking: true });`
)

var (
	page = fmt.Sprintf(`<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<title>Añejo API</title>
<link rel="stylesheet" href="%[1]s/swagger-ui.css" integrity="sha384-wxLW6kwyHktdDGr6Pv1zgm/VGJh99lfUbzSn6HNHBENZlCN7W602k9VkGdxuFvPn" crossorigin="anonymous">
</head>
<body>
<div id="ui"></div>
<script src="%[1]s/swagger-ui-bundle.js" integrity="sha384-wmyclcVGX/WhUkdkATwhaK1X1JtiNrr2EoYJ+diV3vj4v6OC5yCeSu+yW13SYJep" crossorigin="anonymous"></script>
<script>%[2]s</script>
</body>
</html>`, swaggerBase, initScript)

	scriptHash = sha256.Sum256([]byte(initScript))
	policy     = fmt.Sprintf("default-src 'none'; script-src https://unpkg.com 'sha256-%s'; style-src https://unpkg.com 'unsafe-inline'; img-src data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'",
		base64.StdEncoding.EncodeToString(scriptHash[:]))
)

func Routes(r chi.Router) {
	r.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(api.OpenAPISpec)
	})
	r.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", policy)
		_, _ = w.Write([]byte(page))
	})
}
