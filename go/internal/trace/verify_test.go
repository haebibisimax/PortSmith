package trace

import (
	"strings"
	"testing"
)

func TestVerifyCleanStream(t *testing.T) {
	in := strings.Join([]string{
		Magic,
		"V1 100 > s1 redis 3 " + b64("GET"),
		"V1 200 < s1 redis 4 " + b64("PONG"),
		"V1 300 > s2 http 3 " + b64("GET"),
	}, "\n") + "\n"

	rep, err := Verify(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !rep.OK() {
		t.Fatalf("unexpected findings: %v", rep.Findings)
	}
	if rep.Records != 3 || rep.Sessions != 2 {
		t.Errorf("records=%d sessions=%d, want 3 and 2", rep.Records, rep.Sessions)
	}
	if len(rep.Protocols) != 2 || rep.Protocols[0] != "http" || rep.Protocols[1] != "redis" {
		t.Errorf("protocols = %v, want [http redis]", rep.Protocols)
	}
}

func TestVerifyKeepsGoingAfterABadLine(t *testing.T) {
	in := strings.Join([]string{
		"V1 100 > s1 redis 3 " + b64("GET"),
		"V1 notanumber > s1 redis 3 " + b64("GET"),
		"V1 200 < s1 redis 2 " + b64("OK"),
	}, "\n") + "\n"

	rep, err := Verify(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(rep.Findings) != 1 {
		t.Fatalf("findings = %v, want one", rep.Findings)
	}
	f := rep.Findings[0]
	if f.Kind != FindingMalformedLine || f.Line != 2 {
		t.Errorf("finding = %+v, want malformed-line on line 2", f)
	}
	// the good lines after the bad one are still counted
	if rep.Records != 2 {
		t.Errorf("records = %d, want 2", rep.Records)
	}
}

func TestVerifyReportsTimestampOrderPerSession(t *testing.T) {
	in := strings.Join([]string{
		"V1 200 > s1 redis 3 " + b64("GET"),
		"V1 100 < s1 redis 2 " + b64("OK"), // backwards within s1
		"V1 50 > s2 http 3 " + b64("GET"),  // s2 is independent
	}, "\n") + "\n"

	rep, err := Verify(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(rep.Findings) != 1 {
		t.Fatalf("findings = %v, want one", rep.Findings)
	}
	f := rep.Findings[0]
	if f.Kind != FindingTimestampOrder || f.Session != "s1" || f.Line != 2 {
		t.Errorf("finding = %+v, want timestamp-order in s1 on line 2", f)
	}
}

func b64(s string) string {
	return base64EncodeFor(s)
}
