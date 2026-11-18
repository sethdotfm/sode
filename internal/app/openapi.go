package app

import (
	_ "embed"
	"net/http"
)

// openapi.yaml is the hand-written contract. It is served open (it describes the API, it
// is not the API), with no key. TestOpenAPIDocumentsEveryRoute keeps it in
// step with apiRoutes.
//
//go:embed openapi.yaml
var openAPIFile []byte

func serveOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Write(openAPIFile)
}
