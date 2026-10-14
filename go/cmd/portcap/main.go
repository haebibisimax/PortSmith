// Command portcap captures and normalizes protocol traffic into the portsmith
// line-based trace format.
//
// Subcommands:
//
//	portcap capture   -listen :9000 -target host:6379 -proto redis -out cap.trace
//	portcap normalize -in session.log -proto http -out out.trace
//	portcap stats     -in cap.trace
//	portcap version
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/portsmith/portcap/internal/capture"
	"github.com/portsmith/portcap/internal/normalize"
	"github.com/portsmith/portcap/internal/trace"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "capture":
		err = cmdCapture(os.Args[2:])
	case "normalize":
		err = cmdNormalize(os.Args[2:])
	case "stats":
		err = cmdStats(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("portcap %s\n", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "portcap: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "portcap:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `portcap - capture & normalize protocol traffic (portsmith trace v1)

usage:
  portcap capture   -listen ADDR -target ADDR [-proto NAME] [-out FILE] [-max N]
  portcap normalize -in FILE [-proto NAME] [-session ID] [-out FILE]
  portcap stats     -in FILE
  portcap version

examples:
  portcap capture -listen :9000 -target 127.0.0.1:6379 -proto redis -out cap.trace
  portcap normalize -in session.log -proto http -out http.trace
  portcap stats -in cap.trace
`)
}

func cmdCapture(args []string) error {
	fs := flag.NewFlagSet("capture", flag.ExitOnError)
	listen := fs.String("listen", ":9000", "address to listen on")
	target := fs.String("target", "", "upstream target address (host:port)")
	proto := fs.String("proto", "tcp", "protocol hint recorded in the trace")
	out := fs.String("out", "-", "output trace file (- for stdout)")
	max := fs.Int("max", 0, "stop after N connections (0 = unlimited)")
	_ = fs.Parse(args)

	if *target == "" {
		return fmt.Errorf("capture: -target is required")
	}
	w, closeOut, err := openOut(*out)
	if err != nil {
		return err
	}
	defer closeOut()

	tw := trace.NewWriter(w, false)
	defer tw.Flush()

	proxy := capture.New(*listen, *target, *proto, tw)
	fmt.Fprintf(os.Stderr, "# capturing %s -> %s (proto=%s) to %s\n", *listen, *target, *proto, *out)
	return proxy.Serve(*max)
}

func cmdNormalize(args []string) error {
	fs := flag.NewFlagSet("normalize", flag.ExitOnError)
	in := fs.String("in", "-", "input raw session log (- for stdin)")
	proto := fs.String("proto", "raw", "protocol hint recorded in the trace")
	session := fs.String("session", "norm0001", "session id for produced records")
	out := fs.String("out", "-", "output trace file (- for stdout)")
	start := fs.Int64("start", time.Now().UnixNano(), "start timestamp (unix nanos)")
	step := fs.Int64("step", int64(time.Millisecond), "timestamp step between records (nanos)")
	_ = fs.Parse(args)

	r, closeIn, err := openIn(*in)
	if err != nil {
		return err
	}
	defer closeIn()

	records, err := normalize.FromRawLog(r, *proto, *session, *start, *step)
	if err != nil {
		return err
	}

	w, closeOut, err := openOut(*out)
	if err != nil {
		return err
	}
	defer closeOut()

	tw := trace.NewWriter(w, false)
	for _, rec := range records {
		if err := tw.Write(rec); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "# normalized %d records\n", len(records))
	return nil
}

func cmdStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	in := fs.String("in", "-", "input trace file (- for stdin)")
	_ = fs.Parse(args)

	r, closeIn, err := openIn(*in)
	if err != nil {
		return err
	}
	defer closeIn()

	s, err := normalize.Compute(trace.NewReader(r))
	if err != nil {
		return err
	}
	fmt.Printf("records:   %d\n", s.Records)
	fmt.Printf("requests:  %d\n", s.Requests)
	fmt.Printf("responses: %d\n", s.Responses)
	fmt.Printf("sessions:  %d\n", s.Sessions)
	fmt.Printf("bytes:     %d\n", s.TotalBytes)
	fmt.Printf("duration:  %s\n", s.Duration())
	fmt.Println("protocols:")
	for _, k := range s.SortedProtos() {
		fmt.Printf("  %-8s %d\n", k, s.ByProto[k])
	}
	return nil
}

func openIn(path string) (io.Reader, func(), error) {
	if path == "-" || path == "" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

func openOut(path string) (io.Writer, func(), error) {
	if path == "-" || path == "" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}
