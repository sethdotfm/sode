package swp08

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

// Each log line of a link names its router, read at the time of the line, so a new title shows at once.
func TestLogLinesNameTheRouter(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	c := NewClient("10.0.0.9:8910", nil)
	c.logf("connected to %s", "10.0.0.9:8910")
	name := "B Studio B"
	c.Name = func() string { return name }
	c.logf("connected to %s", "10.0.0.9:8910")
	name = "B Edit"
	c.logf("no ACK for cmd 0x%02X", 2)
	out := buf.String()
	for _, want := range []string{"swp08: connected to 10.0.0.9:8910", "swp08: B Studio B: connected to 10.0.0.9:8910", "swp08: B Edit: no ACK for cmd 0x02"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in:\n%s", want, out)
		}
	}
}
