package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// The /log downloads (docs/agents/comm-log.md). The server sends the rows and the page makes the
// file, with its own filter, sort and words. The server only takes out what the reader's Role
// cannot see, so Settings saves values follow that Role.

const (
	logDownloadReqMax = 16 << 10
	partLog           = "log"
	partUptimeStates  = "uptimeStates"
	partUptimeMinutes = "uptimeMinutes"
)

// logDownloadReq: For is the button, "full" or "filtered". It only changes the download row.
type logDownloadReq struct {
	Parts []string `json:"parts"`
	For   string   `json:"for"`
}

// logBuild: what the parts of one reply share. rows counts the Log rows sent.
type logBuild struct {
	a     *App
	admin bool
	req   logDownloadReq
	rows  int
	first time.Time
	last  time.Time
}

// logParts: a new part is a row here. The page names parts by key, so it needs no new route.
// Each part writes one JSON value.
var logParts = map[string]func(b *logBuild, w io.Writer) error{
	partLog:           (*logBuild).writeLog,
	partUptimeStates:  (*logBuild).writeUptimeStates,
	partUptimeMinutes: (*logBuild).writeUptimeMinutes,
}

// eachLogRow calls fn for every Log row, oldest first. Each Settings saves row is below its anchor,
// as on the page. Its values are as the reader's Role sees them.
func (b *logBuild) eachLogRow(fn func(Sent) error) error {
	saves := b.a.saves.Between(-1<<62, b.a.sent.Next())
	slices.Reverse(saves)
	i := 0
	emit := func(e Sent) error {
		return fn(forReader([]Sent{e}, b.admin)[0])
	}
	err := b.a.sent.Each(func(e Sent) error {
		for ; i < len(saves) && saves[i].Seq <= e.Seq; i++ {
			if err := emit(saves[i]); err != nil {
				return err
			}
		}
		return emit(e)
	})
	if err != nil {
		return err
	}
	for ; i < len(saves); i++ {
		if err := emit(saves[i]); err != nil {
			return err
		}
	}
	return nil
}

func (b *logBuild) count(e Sent) {
	if b.rows == 0 || e.Time.Before(b.first) {
		b.first = e.Time
	}
	if b.rows == 0 || e.Time.After(b.last) {
		b.last = e.Time
	}
	b.rows++
}

// writeJSONList writes a JSON array one item at a time, so the whole log is never in memory.
func writeJSONList[T any](w io.Writer, each func(func(T) error) error) error {
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	sep := ""
	err := each(func(v T) error {
		j, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, sep); err != nil {
			return err
		}
		sep = ","
		_, err = w.Write(j)
		return err
	})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "]")
	return err
}

func (b *logBuild) writeLog(w io.Writer) error {
	return writeJSONList(w, func(fn func(Sent) error) error {
		return b.eachLogRow(func(e Sent) error {
			b.count(e)
			return fn(e)
		})
	})
}

func (b *logBuild) writeUptimeStates(w io.Writer) error {
	trans, _ := b.a.firstUnit().up.Record()
	return writeJSONList(w, func(fn func(UptimeTrans) error) error {
		for _, t := range trans {
			if err := fn(t); err != nil {
				return err
			}
		}
		return nil
	})
}

// uptimeMinute: one minute with a count, m in minutes since 1970.
type uptimeMinute struct {
	M    int64 `json:"m"`
	Bad  int64 `json:"bad"`
	Slow int64 `json:"slow"`
}

func (b *logBuild) writeUptimeMinutes(w io.Writer) error {
	_, mins := b.a.firstUnit().up.Record()
	return writeJSONList(w, func(fn func(uptimeMinute) error) error {
		for _, m := range mins {
			if err := fn(uptimeMinute(m)); err != nil {
				return err
			}
		}
		return nil
	})
}

// newLogBuild checks the request. Parts keep the request's order, without repeats.
func (a *App) newLogBuild(req logDownloadReq, admin bool) (*logBuild, error) {
	if len(req.Parts) == 0 {
		return nil, errors.New("name at least one part")
	}
	if req.For != "full" && req.For != "filtered" {
		return nil, errors.New(`for must be "full" or "filtered"`)
	}
	var parts []string
	for _, p := range req.Parts {
		if _, ok := logParts[p]; !ok {
			return nil, fmt.Errorf("no part %q", p)
		}
		if !slices.Contains(parts, p) {
			parts = append(parts, p)
		}
	}
	req.Parts = parts
	return &logBuild{a: a, admin: admin, req: req}, nil
}

// write: one JSON object with a key for each part.
func (b *logBuild) write(w io.Writer) error {
	for i, p := range b.req.Parts {
		sep := ","
		if i == 0 {
			sep = "{"
		}
		key, _ := json.Marshal(p)
		if _, err := io.WriteString(w, sep+string(key)+":"); err != nil {
			return err
		}
		if err := logParts[p](b, w); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "}")
	return err
}

// note: the download row's Note. Times are UTC. For filtered rows the page applies its filters to
// every row it gets. So the count is the rows sent, not the rows in the file.
func (b *logBuild) note() string {
	stamp := func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05") }
	rows := fmt.Sprintf("%d Log rows", b.rows)
	if b.rows == 1 {
		rows = "1 Log row"
	}
	text := "full log, " + rows
	if b.req.For == "filtered" {
		text = "filtered rows, " + rows + " sent to the page"
	}
	if b.rows > 0 {
		text += fmt.Sprintf(", %s to %s UTC", stamp(b.first), stamp(b.last))
	}
	return text
}

// httpLogReader passes whoever can open /log, as admit judges a socket for the log rule. by
// names the reader for the download row: the user, else the address.
func (a *App) httpLogReader(w http.ResponseWriter, r *http.Request) (by string, admin, ok bool) {
	u, signed, cfg, ok := a.httpSigned(w, r)
	switch {
	case !ok:
		return "", false, false
	case len(cfg.Users) == 0:
		return clientIP(r, a.r().proxies), true, true
	case signed:
		role, _ := cfg.ActsAs(u)
		return u.Name, role == config.RoleAdministrator, true
	case cfg.Locks.Log:
		apiFail(w, http.StatusUnauthorized, "sign_in", "sign in on this browser first")
		return "", false, false
	}
	return clientIP(r, a.r().proxies), cfg.GuestRole == config.RoleAdministrator, true
}

func (a *App) serveLogDownload(w http.ResponseWriter, r *http.Request) {
	by, admin, ok := a.httpLogReader(w, r)
	if !ok {
		return
	}
	body, ok := readBody(w, r, logDownloadReqMax, "download request")
	if !ok {
		return
	}
	var req logDownloadReq
	if err := json.Unmarshal(body, &req); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "the download request is not valid JSON")
		return
	}
	b, err := a.newLogBuild(req, admin)
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := b.write(w); err != nil {
		// The status went out with the first bytes. The cut reply is not valid JSON, so the page fails the download.
		log.Printf("log download: %v", err)
		a.commandBy(by, "download", "error", b.note(), err)
		return
	}
	a.commandBy(by, "download", "ok", b.note(), nil)
}
