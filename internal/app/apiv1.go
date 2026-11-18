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

// REST under /api/v1. Every route is one apiRoute in apiRoutes, added by registerAPI from
// the init of its area's file (apireads.go, apihistory.go, ...). mountAPI turns the table
// into mux patterns, each wrapped in the same steps: API switch, Bearer auth (apiAuth),
// rate limit, scope, optional read row. A handler then receives the resolved caller.
// Numbers on the wire are the router's own (0-based, add numberOffset to display them) and
// levels are UI levels, the same as the rights lists. Keep docs in step: openapi.yaml.

const apiBase = "/api/v1"

// apiCtx is who a request is, resolved one time by apiAuth. Writes and the stream reuse it:
// Actor goes in History and the log, and the take path reads Rights through client().
type apiCtx struct {
	Actor  Actor
	Rights config.Rights
	Scopes []string
	Key    *apiKey // nil for a keyless Guest call
	Panel  string  // the panel whose own API client this is, else ""
	IP     string

	// sink gets every reply of a take run for this request instead of a.clients. A REST
	// request is not a registered connection and must not see broadcasts.
	sink func([]byte)
}

func (c *apiCtx) has(scope string) bool { return slices.Contains(c.Scopes, scope) }

// missing is the first scope of the space-separated list the caller lacks, or "".
func (c *apiCtx) missing(scopes string) string {
	for _, s := range strings.Fields(scopes) {
		if !c.has(s) {
			return s
		}
	}
	return ""
}

// client is the take path's view of this caller (see rights.go). It has no send channel:
// deliver answers through sink.
func (c *apiCtx) client() *client { return &client{ip: c.IP, api: c} }

// apiHandler runs after auth, rate limit and scope checks passed.
type apiHandler func(a *App, w http.ResponseWriter, r *http.Request, cl *apiCtx)

// apiRoute: Scope "" means any authenticated caller (none today); several scopes are
// separated by spaces and the caller needs all of them. Path includes apiBase.
type apiRoute struct {
	Method, Path, Scope string
	Handle              apiHandler
}

// scopeOpen as a route's Scope skips the key check: the handler gets a nil apiCtx. Only the setup-link
// exchange and the keyless announcement (panelkeyless.go) use it. The exchange is how a tablet gets its
// key, and it throttles with authWait. The announcement grants nothing and is bounded only by keylessMax.
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

// apiJSONTagged is apiJSON for a read a page repeats. The ETag hashes the body the caller would get, so a Rights or Default artwork
// change moves it. A match answers 304 with no body.
func apiJSONTagged(w http.ResponseWriter, r *http.Request, v any) {
	apiJSONTaggedBy(w, r, v, nil)
}

// apiJSONTaggedBy tags v by the JSON of tagOf, when tagOf is not nil. tagOf leaves out a field that moves
// with time alone, so an unchanged read still gets 304.
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

// apiWriteTagged writes a body already built and tagged, or 304 when the caller has it.
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

// tagMatches: a weak tag compares equal to its strong form, as RFC 9110 says for If-None-Match.
func tagMatches(header, tag string) bool {
	for _, t := range strings.Split(header, ",") {
		t = strings.TrimPrefix(strings.TrimSpace(t), "W/")
		if t == "*" || t == tag {
			return true
		}
	}
	return false
}

// apiCreated is apiJSON with status 201; headers must precede the status line.
func apiCreated(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v)
}

// readBody reads a request body of at most limit bytes. what names it in the 413 message.
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

// pathMatches compares a route path with a request path; a {name} segment matches any one segment.
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
		if rt.Scope == scopeOpen {
			if !a.r().cfg.API.Enabled {
				apiFail(w, http.StatusServiceUnavailable, "api_off", "the API is off; an administrator turns it on in Settings")
				return
			}
			rt.Handle(a, w, r, nil)
			return
		}
		cl, ok := a.apiAuth(w, r)
		if !ok {
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
