package app

import (
	"archive/zip"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// The /log downloads (docs/agents/comm-log.md). The server builds each file for its reader, so
// Settings saves values follow the reader's Role. One part is a plain file; more are a zip.

const (
	logDownloadReqMax = 16 << 10
	partLog           = "log"
	partUptimeStates  = "uptimeStates"
	partUptimeMinutes = "uptimeMinutes"
)

// logDownloadReq: Filter nil is the full log, oldest first. With a Filter, rows go in Sort order.
type logDownloadReq struct {
	Parts  []string   `json:"parts"`
	Filter *logFilter `json:"filter"`
	Sort   logSort    `json:"sort"`
}

// logBuild: what the parts of one download share. rows counts the Log rows written.
type logBuild struct {
	a     *App
	rt    *runtime
	admin bool
	req   logDownloadReq
	pass  func(Sent) bool
	rows  int
	first time.Time
	last  time.Time
}

type logPart struct {
	file  string
	write func(b *logBuild, w io.Writer) error
}

// logParts: a new part is a row here. The page names parts by key, so it needs no new route.
var logParts = map[string]logPart{
	partLog:           {"log.csv", (*logBuild).writeLog},
	partUptimeStates:  {"uptime-states.csv", (*logBuild).writeUptimeStates},
	partUptimeMinutes: {"uptime-minutes.csv", (*logBuild).writeUptimeMinutes},
}

// logCSVCols: CSV_COLS of the page before the server built the file. Keep the names and order,
// since spreadsheets made from older files use them.
var logCSVCols = []string{"time", "seq", "message", "cmd", "level", "router level", "destination", "destination name",
	"source", "source name", "for", "by", "result", "tries", "error", "note", "ack ms", "bytes"}

var csvNumber = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// csvCell: a leading = + - @ is defused, so a spreadsheet does not run a router-supplied name as a formula.
func csvCell(s string) string {
	if s != "" && strings.ContainsAny(s[:1], "=+-@\t\r") && !csvNumber.MatchString(s) {
		s = "'" + s
	}
	if strings.ContainsAny(s, "\",\r\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func writeCSVRow(w io.Writer, cells []string) error {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = csvCell(c)
	}
	_, err := io.WriteString(w, strings.Join(out, ",")+"\r\n")
	return err
}

func (b *logBuild) csvRow(e Sent) []string {
	num := func(n int) string {
		if n < 0 {
			return ""
		}
		return strconv.Itoa(n + b.rt.cfg.NumberOffset)
	}
	cmd, wire, ack := "", "", ""
	if !logEventKinds[e.Kind] {
		cmd = fmt.Sprintf("0x%02X", e.Cmd)
	}
	if e.Wire >= 0 {
		wire = strconv.Itoa(e.Wire)
	}
	if e.AckMs != 0 {
		ack = strconv.FormatFloat(e.AckMs, 'f', -1, 64)
	}
	why := ""
	if e.Why != "" {
		why = logWhyText(e.Why)
	}
	return []string{e.Time.Format(time.RFC3339Nano), strconv.FormatInt(e.Seq, 10), logKindText(e.Kind), cmd,
		b.rt.byUI[e.Level].Label, wire, num(e.Dst), e.DstName, num(e.Src), e.SrcName, why, e.By, e.Result,
		strconv.Itoa(e.Tries), e.Err, logNote(e), ack, e.Hex}
}

// logNote: the row's note, then for a Settings saves row its summary and one line per field.
func logNote(e Sent) string {
	lines := []string{}
	if e.Note != "" {
		lines = append(lines, e.Note)
	}
	if s := e.Save; s != nil {
		lines = append(lines, saveSummary(*s))
		for _, f := range s.Fields {
			lines = append(lines, savedFieldLine(f))
		}
		for _, t := range s.Interruptions {
			lines = append(lines, "Interruption: "+t)
		}
	}
	return strings.Join(lines, "\n")
}

// showValue: show() in web/logsaves.js.
func showValue(v json.RawMessage) string {
	switch s := string(v); s {
	case "true":
		return "On"
	case "false":
		return "Off"
	case "", "null", `""`:
		return "Blank"
	default:
		return s
	}
}

func showList(vs []json.RawMessage) string {
	out := make([]string, len(vs))
	for i, v := range vs {
		var s string
		if json.Unmarshal(v, &s) == nil {
			out[i] = s
		} else {
			out[i] = string(v)
		}
	}
	return strings.Join(out, ", ")
}

// savedFieldLine: "Section: label: old → new". A secret, or a value hidden from the reader's Role, is "changed".
func savedFieldLine(f SavedField) string {
	var what string
	switch {
	case f.Changed:
		what = "changed"
	case f.Added != nil || f.Removed != nil:
		var parts []string
		if len(f.Added) > 0 {
			parts = append(parts, "added "+showList(f.Added))
		}
		if len(f.Removed) > 0 {
			parts = append(parts, "removed "+showList(f.Removed))
		}
		what = strings.Join(parts, "; ")
	default:
		what = showValue(f.Old) + " → " + showValue(f.New)
	}
	text := f.Label + ": " + what
	if f.Section != "" {
		text = f.Section + ": " + text
	}
	if f.Note != "" {
		text += " (" + f.Note + ")"
	}
	return text
}

// importCategories: IMPORT_CAT in web/logsaves.js.
var importCategories = map[string]string{"settings": "Settings", "routers": "Routers", "users": "Users", "presets": "Presets",
	"filterSets": "Filter sets", "panels": "Panels", "apiClients": "API clients"}

// saveSummary: saveSummary in web/logsaves.js.
func saveSummary(s SettingsSave) string {
	plural := func(n int, one, many string) string {
		if n == 1 {
			return "1 " + one
		}
		return strconv.Itoa(n) + " " + many
	}
	var parts []string
	if im := s.Import; im != nil {
		parts = append(parts, importText(im))
	} else {
		parts = append(parts, plural(len(s.Fields), "field", "fields"))
	}
	if s.Critical {
		parts = append(parts, "Critical")
	}
	if s.Approver != "" {
		parts = append(parts, "approved by "+s.Approver)
	}
	if s.Preset != "" {
		parts = append(parts, "preset "+s.Preset)
	}
	if n := len(s.Interruptions) + s.HiddenInterruptions; n > 0 {
		parts = append(parts, plural(n, "Interruption", "Interruptions"))
	}
	return strings.Join(parts, ", ")
}

func importText(im *ImportSave) string {
	var cats []string
	for _, c := range im.Categories {
		name := cmp.Or(importCategories[c.Category], c.Category)
		if c.Refused {
			cats = append(cats, name+" not imported")
			continue
		}
		var n []string
		for _, p := range []struct {
			v int
			w string
		}{{c.Added, "added"}, {c.Updated, "updated"}, {c.Removed, "removed"}, {c.Kept, "kept"}, {c.Broken, "Broken"}} {
			if p.v != 0 {
				n = append(n, fmt.Sprintf("%d %s", p.v, p.w))
			}
		}
		cats = append(cats, name+" "+cmp.Or(strings.Join(n, ", "), "no change"))
	}
	mode := map[string]string{"add": "Add import", "replace": "Replace import"}[im.Mode]
	return cmp.Or(mode, "Import") + ": " + cmp.Or(strings.Join(cats, "; "), "nothing")
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

func (b *logBuild) writeLog(w io.Writer) error {
	if err := writeCSVRow(w, logCSVCols); err != nil {
		return err
	}
	if b.pass == nil {
		return b.eachLogRow(func(e Sent) error {
			b.count(e)
			return writeCSVRow(w, b.csvRow(e))
		})
	}
	var rows []Sent
	if err := b.eachLogRow(func(e Sent) error {
		if b.pass(e) {
			rows = append(rows, e)
		}
		return nil
	}); err != nil {
		return err
	}
	sortLogRows(rows, b.req.Sort)
	for _, e := range rows {
		b.count(e)
		if err := writeCSVRow(w, b.csvRow(e)); err != nil {
			return err
		}
	}
	return nil
}

func uptimeStamp(ms int64) string { return time.UnixMilli(ms).UTC().Format("2006-01-02T15:04:05.000Z") }

// uptimeStateNote: the graph's words for each state.
var uptimeStateNote = map[string]string{upUp: "Router link up", upDown: "Router link down or connecting", upOff: "Sode not running"}

func (b *logBuild) writeUptimeStates(w io.Writer) error {
	trans, _ := b.a.up.Record()
	if err := writeCSVRow(w, []string{"time", "state", "note"}); err != nil {
		return err
	}
	for _, t := range trans {
		if err := writeCSVRow(w, []string{uptimeStamp(t.T), t.S, uptimeStateNote[t.S]}); err != nil {
			return err
		}
	}
	return nil
}

func (b *logBuild) writeUptimeMinutes(w io.Writer) error {
	_, mins := b.a.up.Record()
	if err := writeCSVRow(w, []string{"minute", "bad", "slow"}); err != nil {
		return err
	}
	for _, m := range mins {
		if err := writeCSVRow(w, []string{uptimeStamp(m.M * 60_000), strconv.FormatInt(m.Bad, 10), strconv.FormatInt(m.Slow, 10)}); err != nil {
			return err
		}
	}
	return nil
}

// newLogBuild checks the request. Parts keep logParts' order of the request, without repeats.
func (a *App) newLogBuild(req logDownloadReq, admin bool) (*logBuild, error) {
	if len(req.Parts) == 0 {
		return nil, errors.New("name at least one part")
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
	b := &logBuild{a: a, rt: a.r(), admin: admin, req: req}
	if req.Filter != nil {
		pass, err := req.Filter.compile(b.rt.cfg.NumberOffset)
		if err != nil {
			return nil, err
		}
		b.pass = pass
	}
	return b, nil
}

// write: one part as itself, more parts as a zip.
func (b *logBuild) write(w io.Writer) error {
	if len(b.req.Parts) == 1 {
		return logParts[b.req.Parts[0]].write(b, w)
	}
	z := zip.NewWriter(w)
	for _, p := range b.req.Parts {
		part := logParts[p]
		f, err := z.CreateHeader(&zip.FileHeader{Name: part.file, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			return err
		}
		if err := part.write(b, f); err != nil {
			return err
		}
	}
	return z.Close()
}

// note: the download row's Note. Times are UTC.
func (b *logBuild) note() string {
	stamp := func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05") }
	rows := fmt.Sprintf("%d Log rows", b.rows)
	if b.rows == 1 {
		rows = "1 Log row"
	}
	if f := b.req.Filter; f != nil {
		from, to := "the start", "now"
		if f.From != nil {
			from = stamp(time.UnixMilli(*f.From))
		}
		if f.To != nil {
			to = stamp(time.UnixMilli(*f.To))
		}
		return fmt.Sprintf("filtered rows, from %s to %s UTC, %s", from, to, rows)
	}
	text := "full log, " + rows
	if b.rows > 0 {
		text += fmt.Sprintf(", %s to %s UTC", stamp(b.first), stamp(b.last))
	}
	return text
}

// httpLogReader passes whoever can open /log, as admit judges a socket for the log rule. by
// names the reader for the download row: the user, else the address.
func (a *App) httpLogReader(w http.ResponseWriter, r *http.Request) (by string, admin, ok bool) {
	rt := a.r()
	cfg := rt.cfg
	ip := clientIP(r, rt.proxies)
	if a.setupPendingCfg(cfg) {
		apiFail(w, http.StatusServiceUnavailable, "setup_pending", errSetup.Error())
		return "", false, false
	}
	if len(cfg.Users) == 0 {
		return ip, true, true
	}
	tok := r.Header.Get(sessionHeader)
	if u, signed := a.tokens.user(tok, cfg); signed {
		if a.tokens.locked(tok) {
			apiFail(w, http.StatusForbidden, "locked", errPageLocked.Error())
			return "", false, false
		}
		role, _ := cfg.ActsAs(u)
		return u.Name, role == config.RoleAdministrator, true
	}
	if cfg.Locks.Log {
		apiFail(w, http.StatusUnauthorized, "sign_in", "sign in on this browser first")
		return "", false, false
	}
	return ip, cfg.GuestRole == config.RoleAdministrator, true
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
	name, typ := "sode-log-"+time.Now().UTC().Format("20060102-1504"), "text/csv; charset=utf-8"
	if len(b.req.Parts) > 1 {
		name, typ = name+".zip", "application/zip"
	} else {
		name += ".csv"
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	if err := b.write(w); err != nil {
		// The status went out with the first bytes, so the cut file is all the page gets.
		log.Printf("log download: %v", err)
		a.commandBy(by, "download", "error", b.note(), err)
		return
	}
	a.commandBy(by, "download", "ok", b.note(), nil)
}
