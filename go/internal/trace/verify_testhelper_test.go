package trace

import "encoding/base64"

// base64EncodeFor is a test helper that encodes a string so fixtures stay
// readable in the test source.
func base64EncodeFor(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
