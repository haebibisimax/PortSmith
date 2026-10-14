package trace

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// FindingKind classifies a structural problem found by Verify.
type FindingKind string

const (
	// FindingMalformedLine is a line that does not decode as a record.
	FindingMalformedLine FindingKind = "malformed-line"
	// FindingTimestampOrder is a record whose timestamp is older than one
	// already seen in the same session.
	FindingTimestampOrder FindingKind = "timestamp-order"
)

// Finding is one structural problem, with enough context to find it again.
type Finding struct {
	Kind    FindingKind
	Line    int
	Session string
	Detail  string
}

func (f Finding) String() string {
	if f.Session != "" {
		return fmt.Sprintf("%s: line %d (session %s): %s", f.Kind, f.Line, f.Session, f.Detail)
	}
	return fmt.Sprintf("%s: line %d: %s", f.Kind, f.Line, f.Detail)
}

// VerifyReport summarises a structural check of a trace stream.
type VerifyReport struct {
	Records   int
	Sessions  int
	Protocols []string
	Findings  []Finding
}

// OK reports whether the stream had no structural problems.
func (v VerifyReport) OK() bool { return len(v.Findings) == 0 }

// Verify reads a trace stream and reports every structural problem it finds
// rather than stopping at the first one: a broken export usually has more than
// one, and the reader in this package deliberately refuses to guess past a bad
// line, so the count is what tells you how much of the file is unusable.
func Verify(r io.Reader) (VerifyReport, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	var rep VerifyReport
	lastTS := make(map[string]int64)
	protos := make(map[string]bool)

	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimRight(sc.Text(), "\r")
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		rec, err := DecodeLine(text, line)
		if err != nil {
			rep.Findings = append(rep.Findings, Finding{
				Kind:   FindingMalformedLine,
				Line:   line,
				Detail: err.Error(),
			})
			continue
		}
		rep.Records++
		protos[rec.Proto] = true

		if prev, seen := lastTS[rec.Session]; seen && rec.TimestampNanos < prev {
			rep.Findings = append(rep.Findings, Finding{
				Kind:    FindingTimestampOrder,
				Line:    line,
				Session: rec.Session,
				Detail: fmt.Sprintf("timestamp went backwards: %d after %d",
					rec.TimestampNanos, prev),
			})
		}
		if rec.TimestampNanos > lastTS[rec.Session] {
			lastTS[rec.Session] = rec.TimestampNanos
		}
	}
	if err := sc.Err(); err != nil {
		return rep, err
	}

	rep.Sessions = len(lastTS)
	for p := range protos {
		rep.Protocols = append(rep.Protocols, p)
	}
	sort.Strings(rep.Protocols)
	return rep, nil
}
