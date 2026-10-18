package main

import (
	"fmt"
	"io"
	"log/syslog"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"time"
)

// Longest each quoted field may grow once escaped, so a line stays well under
// syslogd's message limit however large the headers a client sends.
const (
	maxLogRequest   = 2000
	maxLogReferer   = 1000
	maxLogUserAgent = 500
)

// openAccessLog returns where -access-log sends request lines: "syslog"
// (facility local0, which production's syslog.conf, managed by webmaster,
// routes to its own file), "stderr",
// or nil for "" (no access log).
func openAccessLog(dest string) (io.Writer, error) {
	switch dest {
	case "":
		return nil, nil
	case "stderr":
		return os.Stderr, nil
	case "syslog":
		return syslog.New(syslog.LOG_LOCAL0|syslog.LOG_INFO, "wintermuteconsulting")
	}
	return nil, fmt.Errorf(`-access-log %q: want "syslog", "stderr" or ""`, dest)
}

// accessLog writes one line per request in the Combined Log Format, which
// GoAccess, GoatCounter and most log tools read as is, with the time taken in
// seconds appended:
//
//	192.0.2.7 - - [18/Oct/2026:12:00:00 +0000] "GET /about HTTP/1.1" 200 5120 "https://example.com/" "Mozilla/5.0 ..." 0.002
//
// The client IP is clientIP's (the proxy's last X-Forwarded-For hop under
// -trust-proxy). Request bodies are never logged, so contact-form text stays
// out of it. /healthz is skipped: relayd checks it every 10 seconds.
func (a *app) accessLog(next http.Handler) http.Handler {
	if a.access == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/healthz" {
			return
		}

		b := make([]byte, 0, 256)
		if ip, err := netip.ParseAddr(a.clientIP(r)); err == nil {
			b = append(b, ip.String()...)
		} else {
			b = append(b, '-') // not an address: a misconfigured proxy, never client text
		}
		b = append(b, " - - ["...)
		b = start.UTC().AppendFormat(b, "02/Jan/2006:15:04:05 -0700")
		b = append(b, "] "...)
		b = appendQuoted(b, r.Method+" "+r.RequestURI+" "+r.Proto, maxLogRequest)
		b = append(b, ' ')
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		b = strconv.AppendInt(b, int64(status), 10)
		b = append(b, ' ')
		if rec.bytes == 0 {
			b = append(b, '-')
		} else {
			b = strconv.AppendInt(b, rec.bytes, 10)
		}
		b = append(b, ' ')
		b = appendQuoted(b, orDash(r.Referer()), maxLogReferer)
		b = append(b, ' ')
		b = appendQuoted(b, orDash(r.UserAgent()), maxLogUserAgent)
		b = append(b, ' ')
		b = strconv.AppendFloat(b, time.Since(start).Seconds(), 'f', 3, 64)
		b = append(b, '\n')
		_, _ = a.access.Write(b) // one Write per line keeps concurrent lines whole
	})
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// appendQuoted appends s in double quotes, escaped the way Apache escapes log
// fields: '"' and '\' get a backslash, and control characters, DEL and every
// non-ASCII byte become \xhh. Whatever a client sends, the field stays on one
// line of printable ASCII and can't forge an entry or end the field early.
// Once the escaped text would pass limit bytes the rest is cut and marked "...".
func appendQuoted(b []byte, s string, limit int) []byte {
	const hex = "0123456789abcdef"
	b = append(b, '"')
	start := len(b)
	for i := 0; i < len(s); i++ {
		c := s[i]
		n := 1
		if c == '"' || c == '\\' {
			n = 2
		} else if c < 0x20 || c >= 0x7f {
			n = 4
		}
		if len(b)-start+n > limit {
			b = append(b, "..."...)
			break
		}
		switch n {
		case 2:
			b = append(b, '\\', c)
		case 4:
			b = append(b, '\\', 'x', hex[c>>4], hex[c&0xf])
		default:
			b = append(b, c)
		}
	}
	return append(b, '"')
}

// statusRecorder notes the status code and body size a handler sends.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 && code >= 200 { // 1xx are interim; the final status follows
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += int64(n)
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
