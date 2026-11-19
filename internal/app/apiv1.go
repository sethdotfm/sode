package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Each area's file adds its routes to apiRoutes from init (registerAPI). mountAPI wraps each in the same steps:
// Bearer auth (apiAuth), rate limit, scope, optional read row. Numbers on the wire are the router's own (0-based;
// add numberOffset to show them) and levels are UI levels. Keep openapi.yaml in step.

const apiBase = "/api/v1"

// apiAuth resolves it one time. Actor goes in History and the log, and the take path reads Rights through client().
type apiCtx struct {
	Actor  Actor
	Rights config.Rights
	Scopes []string
	Key    *apiKey // nil for a keyless Guest call
	Panel  string  // "" unless this is a panel's own API client
	IP     string

	// sink gets every reply of this request's take, not a.clients: a REST request must not see broadcasts.
	sink func([]byte)
}

func (c *apiCtx) has(scope string) bool { return slices.Contains(c.Scopes, scope) }

// "" when the caller has every scope of the space-separated list.
func (c *apiCtx) missing(scopes string) string {
	for _, s := range strings.Fields(scopes) {
		if !c.has(s) {
			return s
		}
	}
	return ""
}

// The take path's view of this caller (rights.go). It has no send channel: deliver answers through sink.
func (c *apiCtx) client() *client { return &client{ip: c.IP, api: c} }

type apiHandler func(a *App, w http.ResponseWriter, r *http.Request, cl *apiCtx)

// Scope "" means any authenticated caller. Several scopes are separated by spaces, and the caller needs all of them.
type apiRoute struct {
	Method, Path, Scope string
	Handle              apiHandler
}

// scopeOpen skips the key check, so the handler gets a nil apiCtx. Only the setup-link exchange (throttled by authWait)
// and the keyless announcement (panelkeyless.go, bounded by keylessMax, grants nothing) use it.
const scopeOpen = "none"

var apiRoutes []apiRoute

func registerAPI(rs ...apiRoute) { apiRoutes = append(apiRoutes, rs...) }

type apiErrBody struct {
	Error apiErr `json:"error"`
}

type apiErr struct {
	Code    string `json:"code"` // machine-readable reason, listed in openapi.yaml
	Message string `json:"message"`
	Suggest string `json:"suggest,omitempty"` // name_taken only: a free name to send instead
}

func apiFail(w http.ResponseWriter, status int, code, msg string) {
	(&apiProblem{status, code, msg}).write(w)
}

func apiJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

// The ETag hashes the body the caller would get, so a change of Rights or Default artwork moves it.
func apiJSONTagged(w http.ResponseWriter, r *http.Request, v any) {
	apiJSONTaggedBy(w, r, v, nil)
}

// tagOf, when not nil, is v without the fields that move with time alone, so an unchanged read still gets 304.
func apiJSONTaggedBy(w http.ResponseWriter, r *http.Request, v, tagOf any) {
	body, err := json.Marshal(v)
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the answer could not be built")
		return
	}
	tagged := body
	if tagOf != nil {
		if tagged, err = json.Marshal(tagOf); err != nil {
			apiFail(w, http.StatusInternalServerError, "internal", "the answer could not be built")
			return
		}
	}
	apiWriteTagged(w, r, body, etagOf(tagged))
}

func etagOf(b []byte) string {
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

func apiWriteTagged(w http.ResponseWriter, r *http.Request, body []byte, tag string) {
	w.Header().Set("ETag", tag)
	w.Header().Set("Cache-Control", "no-store")
	if tagMatches(r.Header.Get("If-None-Match"), tag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
	w.Write([]byte{'\n'})
}

// RFC 9110: for If-None-Match a weak tag equals its strong form.
func tagMatches(header, tag string) bool {
	for _, t := range strings.Split(header, ",") {
		t = strings.TrimPrefix(strings.TrimSpace(t), "W/")
		if t == "*" || t == tag {
			return true
		}
	}
	return false
}

// Headers must precede the status line.
func apiCreated(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v)
}

// what names the body in the 413 message.
func readBody(w http.ResponseWriter, r *http.Request, limit int64, what string) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			apiFail(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("the %s is over %d KiB", what, limit>>10))
		} else {
			apiFail(w, http.StatusBadRequest, "bad_request", "the body could not be read")
		}
		return nil, false
	}
	return body, true
}

func isAPIPath(p string) bool { return strings.HasPrefix(p, apiBase+"/") }

// Unknown paths and wrong methods are answered as JSON. v1 sets no CORS headers.
func (a *App) mountAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET "+apiBase+"/openapi.yaml", serveOpenAPI)
	for _, rt := range apiRoutes {
		mux.HandleFunc(rt.Method+" "+rt.Path, a.apiWrap(rt))
	}
	mux.HandleFunc(apiBase+"/", func(w http.ResponseWriter, r *http.Request) {
		var allow []string
		for _, rt := range apiRoutes {
			if pathMatches(rt.Path, r.URL.Path) && !slices.Contains(allow, rt.Method) {
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

// The router parameter is the router UID, never a prefix, since a prefix can change.
func (a *App) routerOK(w http.ResponseWriter, r *http.Request) bool {
	uid := r.URL.Query().Get("router")
	if uid == "" || uid == a.r().cfg.UID {
		return true
	}
	apiFail(w, http.StatusNotFound, "unknown_router", fmt.Sprintf("no router with UID %q; GET %s/routers lists them", uid, apiBase))
	return false
}

// A {name} segment matches any one segment.
func pathMatches(pattern, path string) bool {
	ps, qs := strings.Split(pattern, "/"), strings.Split(path, "/")
	if len(ps) != len(qs) {
		return false
	}
	for i, p := range ps {
		if p != qs[i] && !(strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") && qs[i] != "") {
			return false
		}
	}
	return true
}

func (a *App) apiWrap(rt apiRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.setupPending() {
			apiFail(w, http.StatusServiceUnavailable, "setup_pending", errSetup.Error())
			return
		}
		if rt.Scope == scopeOpen {
			if a.routerOK(w, r) {
				rt.Handle(a, w, r, nil)
			}
			return
		}
		cl, ok := a.apiAuth(w, r)
		if !ok || !a.routerOK(w, r) {
			return
		}
		if missing := cl.missing(rt.Scope); missing != "" {
			msg := fmt.Sprintf("this key lacks the %q scope", missing)
			a.apiEvent(cl.Actor, "refused", "refused", rt.Method+" "+r.URL.Path, fmt.Errorf("%s", msg))
			apiFail(w, http.StatusForbidden, "scope", msg)
			return
		}
		if rt.Method == http.MethodGet && a.r().cfg.API.LogReads && r.URL.Path != apiBase+"/stream" { // the stream writes streamConnect
			a.apiEvent(cl.Actor, "read", "ok", r.URL.Path, nil)
		}
		rt.Handle(a, w, r, cl)
	}
}
