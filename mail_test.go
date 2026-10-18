package main

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"testing"
	"time"
)

// fakeSMTP is just enough of an SMTP server on loopback to receive one message
// per connection. It offers AUTH but no STARTTLS (send allows that on loopback
// only). reject makes it refuse the recipient.
type fakeSMTP struct {
	addr   string
	got    chan string // MAIL FROM, RCPT TO and the message, joined by \n\n
	reject bool
}

func startFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	s := &fakeSMTP{addr: ln.Addr().String(), got: make(chan string, 10)}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return s
}

func (s *fakeSMTP) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	say := func(l string) { io.WriteString(c, l+"\r\n") }
	var from, to string
	say("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"):
			say("250-fake")
			say("250 AUTH PLAIN")
		case strings.HasPrefix(cmd, "AUTH PLAIN"):
			say("235 ok")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			from = strings.TrimSpace(line[len("MAIL FROM:"):])
			say("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			if s.reject {
				say("550 no such user")
				continue
			}
			to = strings.TrimSpace(line[len("RCPT TO:"):])
			say("250 ok")
		case cmd == "DATA":
			say("354 go on")
			var msg strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				msg.WriteString(strings.TrimPrefix(l, "."))
			}
			s.got <- from + "\n\n" + to + "\n\n" + msg.String()
			say("250 queued")
		case cmd == "QUIT":
			say("221 bye")
			return
		default:
			say("250 ok")
		}
	}
}

func testMailer(addr string) *mailer {
	return &mailer{addr: addr, user: "info@example.com", password: "app-password", from: "info@example.com", to: "inbox@example.com", timeout: 5 * time.Second}
}

func TestContactSendsEmail(t *testing.T) {
	srv := startFakeSMTP(t)
	a := testApp(t)
	a.mail = testMailer(srv.addr)

	f := validForm()
	f.Set("name", "Åsa Ström")
	f.Set("message", "Line one.\n.\nA line that is only a dot must not end the message.")
	if rec := postContact(a.routes(), f, ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("code %d, want 303", rec.Code)
	}
	var got string
	select {
	case got = <-srv.got:
	case <-time.After(5 * time.Second):
		t.Fatal("no message reached the SMTP server")
	}
	parts := strings.SplitN(got, "\n\n", 3)
	if parts[0] != "<info@example.com>" || parts[1] != "<inbox@example.com>" {
		t.Errorf("envelope from %s to %s", parts[0], parts[1])
	}
	msg, err := mail.ReadMessage(strings.NewReader(parts[2]))
	if err != nil {
		t.Fatal(err)
	}
	if rt, err := mail.ParseAddress(msg.Header.Get("Reply-To")); err != nil || rt.Address != f.Get("email") || rt.Name != "Åsa Ström" {
		t.Errorf("Reply-To = %q (%v), want the visitor", msg.Header.Get("Reply-To"), err)
	}
	if subj, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); !strings.Contains(subj, "Åsa Ström") {
		t.Errorf("Subject = %q", subj)
	}
	body, _ := io.ReadAll(quotedprintable.NewReader(msg.Body))
	for _, want := range []string{"Åsa Ström", f.Get("email"), "\r\n.\r\nA line that is only a dot"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("body lacks %q:\n%s", want, body)
		}
	}
}

func TestContactMailHeaderInjection(t *testing.T) {
	m := testMailer("127.0.0.1:25")
	f := contactForm{Name: "Ada\r\nBcc: victim@example.net", Email: "ada@example.com", Company: "X\nBcc: other@example.net", Message: "hello there, world"}
	msg, err := mail.ReadMessage(bytes.NewReader(m.compose(f, "Site", time.Now())))
	if err != nil {
		t.Fatal(err)
	}
	if b := msg.Header.Get("Bcc"); b != "" {
		t.Errorf("a form field added a Bcc header: %q", b)
	}
	for k, v := range msg.Header {
		for _, s := range v {
			if strings.ContainsAny(s, "\r\n") {
				t.Errorf("header %s carries a raw newline: %q", k, s)
			}
		}
	}
}

func TestContactDeliveryFailureKeepsTheForm(t *testing.T) {
	srv := startFakeSMTP(t)
	srv.reject = true
	var logBuf bytes.Buffer
	a := testApp(t)
	a.log = slog.New(slog.NewTextHandler(&logBuf, nil))
	a.mail = testMailer(srv.addr)

	f := validForm()
	rec := postContact(a.routes(), f, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "could not be sent") || !strings.Contains(rec.Body.String(), f.Get("message")) {
		t.Error("the visitor isn't told, or loses what they typed")
	}
	if !strings.Contains(logBuf.String(), "contact delivery failed") || !strings.Contains(logBuf.String(), f.Get("email")) {
		t.Errorf("the undelivered enquiry isn't in the log:\n%s", logBuf.String())
	}
}

func TestMailRefusesPlaintextOffLoopback(t *testing.T) {
	// A server that doesn't offer STARTTLS on a non-loopback name: send must
	// give up before AUTH rather than send the password in the clear.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		io.WriteString(c, "220 fake\r\n")
		r.ReadString('\n')
		io.WriteString(c, "250-fake\r\n250 AUTH PLAIN\r\n")
		r.ReadString('\n')
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	// Dial the loopback listener but present a public name to the client.
	m := testMailer(net.JoinHostPort("smtp.example.com", port))
	if err := m.sendVia(ln.Addr().String(), []byte("x")); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Errorf("err = %v, want a refusal for missing STARTTLS", err)
	}
}

func TestParseMailConfig(t *testing.T) {
	m, err := parseMailConfig("mail.conf", []byte("# Google Workspace\nSMTP_USER = info@example.com\nSMTP_PASSWORD=abcd efgh ijkl mnop\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m.addr != "smtp.gmail.com:465" || m.from != "info@example.com" || m.to != "info@example.com" || m.password != "abcd efgh ijkl mnop" {
		t.Errorf("defaults: %+v", m)
	}
	for _, bad := range []string{
		"SMTP_USER=a@example.com\n",                                     // password missing
		"SMTP_USER=a@example.com\nSMTP_PASWORD=x\n",                     // misspelt
		"SMTP_USER=a@example.com\nSMTP_PASSWORD=x\nSMTP_PASSWORD=y\n",   // twice
		"SMTP_USER=a@example.com\nSMTP_PASSWORD=x\nMAIL_TO=Ada <a@b>\n", // not a plain address
		"SMTP_USER=a@example.com\nSMTP_PASSWORD=x\nSMTP_ADDR=smtp.gmail.com\n",
		"SMTP_USER a@example.com\n",
	} {
		if _, err := parseMailConfig("mail.conf", []byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
