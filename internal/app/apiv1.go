package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// REST under /api/v1. Every route is one apiRoute in apiRoutes, added by registerAPI from
// the init of its area's file (apireads.go, apihistory.go, ...). mountAPI turns the table
// into mux patterns, each wrapped in the same steps: API switch, Bearer auth (apiAuth),
// rate limit, scope, optional read row. A handler then receives the resolved caller.
// Numbers on the wire are the router's own (0-based, add numberOffset to display them) and
// levels are UI levels, the same as the rights lists. Keep docs in step: openapi.yaml.

const apiBase = "/api/v1"

// apiCtx is who a request is, resolved once by apiAuth. Writes (E) and the stream (F) reuse
// it: Actor goes in History and the log, Rights go to the take path through client().
type apiCtx struct {
	Actor  Actor
	Rights config.Rights
	Scopes []string
	Key    *apiKey // nil for a keyless Guest call
	IP     string
}

func (c *apiCtx) has(scope string) bool { return slices.Contains(c.Scopes, scope) }

func (c *apiCtx) caller() *apiCaller { return &apiCaller{Actor: c.Actor, Rights: c.Rights} }

// client is the take path's view of this caller (see rights.go).
func (c *apiCtx) client() *client {
	return &client{ip: c.IP, send: make(chan []byte, 256), api: c.caller()}
}

// apiHandler runs after auth, rate limit and scope checks passed.
type apiHandler func(a *App, w http.ResponseWriter, r *http.Request, cl *apiCtx)

// apiRoute: Scope "" means any authenticated caller (none today). Path includes apiBase.
type apiRoute struct {
	Method, Path, Scope string
	Handle              apiHandler
}

var apiRoutes []apiRoute

func registerAPI(rs ...apiRoute) { apiRoutes = append(apiRoutes, rs...) }

type apiErrBody struct {
	Error apiErr `json:"error"`
}

type apiErr struct {
	Code    string `json:"code"` // machine-readable reason, listed in openapi.yaml
	Message string `json:"message"`
}

func apiFail(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiErrBody{apiErr{code, msg}})
}

func apiJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func isAPIPath(p string) bool { return strings.HasPrefix(p, apiBase+"/") }

// mountAPI adds the table, the open OpenAPI file and a fallback that answers unknown paths
// and wrong methods as JSON. No CORS headers are ever set (v1).
func (a *App) mountAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET "+apiBase+"/openapi.yaml", serveOpenAPI)
	for _, rt := range apiRoutes {
		mux.HandleFunc(rt.Method+" "+rt.Path, a.apiWrap(rt))
	}
	mux.HandleFunc(apiBase+"/", func(w http.ResponseWriter, r *http.Request) {
		var allow []string
		for _, rt := range apiRoutes {
			if rt.Path == r.URL.Path && !slices.Contains(allow, rt.Method) {
				allow = append(allow, rt.Method)
			}
		}
		if len(allow) > 0 {
			w.Header().Set("Allow", strings.Join(allow, ", "))
			apiFail(w, http.StatusMethodNotAllowed, "method_not_allowed", "use "+strings.Join(allow, " or ")+" on this path")
			return
		}
		apiFail(w, http.StatusNotFound, "not_found", "no such API path; see "+apiBase+"/openapi.yaml")
	})
}

func (a *App) apiWrap(rt apiRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cl, ok := a.apiAuth(w, r)
		if !ok {
			return
		}
		if rt.Scope != "" && !cl.has(rt.Scope) {
			msg := fmt.Sprintf("this key lacks the %q scope", rt.Scope)
			a.apiEvent(cl.Actor, "refused", "refused", rt.Method+" "+r.URL.Path, fmt.Errorf("%s", msg))
			apiFail(w, http.StatusForbidden, "scope", msg)
			return
		}
		if rt.Method == http.MethodGet && a.r().cfg.API.LogReads {
			a.apiEvent(cl.Actor, "read", "ok", r.URL.Path, nil)
		}
		rt.Handle(a, w, r, cl)
	}
}
