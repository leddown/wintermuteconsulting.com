package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// mailer delivers contact enquiries over SMTP with the standard library:
// implicit TLS on 465 (or STARTTLS on any other port), then AUTH PLAIN. With
// Google Workspace that is smtp.gmail.com:465 and an app password for the
// mailbox. 465 is the default because STARTTLS on 587 stalled from the Vultr
// production host while 465 went through.
type mailer struct {
	addr     string // host:port
	user     string
	password string
	from     string // envelope and From: address; Gmail requires the account or one of its aliases
	to       string
	timeout  time.Duration // for the whole conversation; must stay under the server's WriteTimeout
}

// parseMailConfig reads the mail settings file: KEY=VALUE lines, # comments.
// Like content.json it is strict, because a typo here would silently lose
// enquiries: unknown or repeated keys and missing values stop the program.
func parseMailConfig(name string, b []byte) (*mailer, error) {
	known := map[string]bool{"SMTP_ADDR": true, "SMTP_USER": true, "SMTP_PASSWORD": true, "MAIL_FROM": true, "MAIL_TO": true}
	v := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, val, ok := strings.Cut(line, "=")
		k, val = strings.TrimSpace(k), strings.TrimSpace(val)
		switch {
		case !ok:
			return nil, fmt.Errorf("%s:%d: want KEY=VALUE", name, n)
		case !known[k]:
			return nil, fmt.Errorf("%s:%d: unknown setting %q", name, n, k)
		case v[k] != "":
			return nil, fmt.Errorf("%s:%d: %s is set twice", name, n, k)
		}
		v[k] = val
	}
	m := &mailer{addr: v["SMTP_ADDR"], user: v["SMTP_USER"], password: v["SMTP_PASSWORD"], from: v["MAIL_FROM"], to: v["MAIL_TO"], timeout: 10 * time.Second}
	if m.addr == "" {
		m.addr = "smtp.gmail.com:465"
	}
	if m.from == "" {
		m.from = m.user
	}
	if m.to == "" {
		m.to = m.from
	}
	if _, _, err := net.SplitHostPort(m.addr); err != nil {
		return nil, fmt.Errorf("%s: SMTP_ADDR: %v", name, err)
	}
	if (m.user == "") != (m.password == "") {
		return nil, fmt.Errorf("%s: set both SMTP_USER and SMTP_PASSWORD, or neither", name)
	}
	for k, a := range map[string]string{"MAIL_FROM": m.from, "MAIL_TO": m.to} {
		if p, err := mail.ParseAddress(a); err != nil || p.Address != a {
			return nil, fmt.Errorf("%s: %s must be a plain address like info@example.com", name, k)
		}
	}
	return m, nil
}

// compose builds the enquiry email. Reply-To is the visitor, so answering it
// in the mailbox goes straight back to them. Every header value that came from
// the form is MIME-encoded, so a newline in a name can't add a header.
func (m *mailer) compose(f contactForm, siteName string, now time.Time) []byte {
	subject := "Website enquiry from " + f.Name
	if f.Company != "" {
		subject += " (" + f.Company + ")"
	}
	id := make([]byte, 12)
	rand.Read(id)
	_, host, _ := strings.Cut(m.from, "@")

	var b bytes.Buffer
	hdr := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	hdr("From", (&mail.Address{Name: siteName + " website", Address: m.from}).String())
	hdr("To", m.to)
	hdr("Reply-To", (&mail.Address{Name: f.Name, Address: f.Email}).String())
	hdr("Subject", mime.QEncoding.Encode("utf-8", subject))
	hdr("Date", now.Format(time.RFC1123Z))
	hdr("Message-ID", "<"+hex.EncodeToString(id)+"@"+host+">")
	hdr("MIME-Version", "1.0")
	hdr("Content-Type", "text/plain; charset=utf-8")
	hdr("Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")

	company := f.Company
	if company == "" {
		company = "(not given)"
	}
	body := fmt.Sprintf("Name:    %s\r\nEmail:   %s\r\nCompany: %s\r\n\r\n%s\r\n\r\n-- \r\nSent from the contact form on the website. Reply to this email to answer %s.\r\n",
		f.Name, f.Email, company, strings.ReplaceAll(strings.ReplaceAll(f.Message, "\r\n", "\n"), "\n", "\r\n"), f.Name)
	qp := quotedprintable.NewWriter(&b)
	_, _ = qp.Write([]byte(body)) // into a bytes.Buffer: can't fail
	_ = qp.Close()
	return b.Bytes()
}

// send runs one SMTP conversation. It refuses to send credentials or an
// enquiry in the clear: the server must offer TLS unless it is on loopback.
func (m *mailer) send(msg []byte) error { return m.sendVia(m.addr, msg) }

// sendVia connects to dial but treats the server as m.addr's host for TLS and
// the plaintext check. The two differ only in tests.
func (m *mailer) sendVia(dial string, msg []byte) error {
	host, port, _ := net.SplitHostPort(m.addr)
	deadline := time.Now().Add(m.timeout)
	d := &net.Dialer{Deadline: deadline}
	tlsConf := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}

	// Each step names itself in the error, so a timeout says where it stalled.
	var conn net.Conn
	var err error
	if port == "465" {
		conn, err = tls.DialWithDialer(d, "tcp", dial, tlsConf)
	} else {
		conn, err = d.Dial("tcp", dial)
	}
	if err != nil {
		return fmt.Errorf("smtp connect: %w", err)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp greeting: %w", err)
	}
	defer c.Close()

	name, err := os.Hostname()
	if err != nil || name == "" {
		name = "localhost"
	}
	if err := c.Hello(name); err != nil {
		return fmt.Errorf("smtp EHLO: %w", err)
	}
	if port != "465" {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsConf); err != nil {
				return fmt.Errorf("smtp STARTTLS: %w", err)
			}
		} else if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return errors.New("smtp: server does not offer STARTTLS")
		}
	}
	if m.user != "" {
		if err := c.Auth(smtp.PlainAuth("", m.user, m.password, host)); err != nil {
			return fmt.Errorf("smtp AUTH: %w", err)
		}
	}
	if err := c.Mail(m.from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := c.Rcpt(m.to); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	return c.Quit()
}
