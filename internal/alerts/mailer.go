package alerts

import (
    "crypto/tls"
    "fmt"
    "net/smtp"
    "os"
    "strings"
)

type smtpConfig struct {
    Host     string
    Port     string
    Username string
    Password string
    From     string
}

var mailCfg smtpConfig
var mailProvider string

// ConfigureMailerFromEnv loads SMTP configuration from environment variables.
// Required: SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, SMTP_FROM
func ConfigureMailerFromEnv() error {
    mailProvider = os.Getenv("MAIL_PROVIDER")
    mailCfg = smtpConfig{
        Host:     os.Getenv("SMTP_HOST"),
        Port:     os.Getenv("SMTP_PORT"),
        Username: os.Getenv("SMTP_USERNAME"),
        Password: os.Getenv("SMTP_PASSWORD"),
        From:     os.Getenv("SMTP_FROM"),
    }
    if mailProvider == "plunk" {
        // Plunk will be configured lazily in sendViaPlunk
        return nil
    }
    if mailCfg.Host == "" || mailCfg.Port == "" || mailCfg.Username == "" || mailCfg.Password == "" || mailCfg.From == "" {
        return fmt.Errorf("smtp not configured: set SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, SMTP_FROM (or set MAIL_PROVIDER=plunk)")
    }
    return nil
}

// SendEmail sends a plain text email using SMTP with TLS.
func SendEmail(to, subject, body string) error {
    if mailCfg.Host == "" && mailProvider == "" {
        _ = ConfigureMailerFromEnv()
    }

    // Route to provider
    if mailProvider == "plunk" || (os.Getenv("PLUNK_API_KEY") != "" && mailProvider == "") {
        return sendViaPlunk(to, subject, body)
    }

    addr := mailCfg.Host + ":" + mailCfg.Port
    // Build message
    msg := ""
    msg += fmt.Sprintf("From: %s\r\n", mailCfg.From)
    msg += fmt.Sprintf("To: %s\r\n", to)
    msg += fmt.Sprintf("Subject: %s\r\n", subject)
    if rt := os.Getenv("MAIL_REPLY_TO"); rt != "" {
        msg += fmt.Sprintf("Reply-To: %s\r\n", rt)
    }
    msg += "MIME-Version: 1.0\r\n"
    contentType := "text/plain"
    lb := strings.ToLower(body)
    if strings.Contains(lb, "<html") || strings.Contains(lb, "<body") || strings.Contains(lb, "<!doctype html") {
        contentType = "text/html"
    }
    msg += fmt.Sprintf("Content-Type: %s; charset=\"utf-8\"\r\n", contentType)
    msg += "\r\n" + body + "\r\n"

    // TLS connection
    tlsConfig := &tls.Config{ServerName: mailCfg.Host}
    conn, err := tls.Dial("tcp", addr, tlsConfig)
    if err != nil {
        return fmt.Errorf("smtp dial: %w", err)
    }
    defer conn.Close()

    c, err := smtp.NewClient(conn, mailCfg.Host)
    if err != nil {
        return fmt.Errorf("smtp client: %w", err)
    }
    defer c.Close()

    auth := smtp.PlainAuth("", mailCfg.Username, mailCfg.Password, mailCfg.Host)
    if err := c.Auth(auth); err != nil {
        return fmt.Errorf("smtp auth: %w", err)
    }
    if err := c.Mail(mailCfg.From); err != nil {
        return fmt.Errorf("smtp mail from: %w", err)
    }
    if err := c.Rcpt(to); err != nil {
        return fmt.Errorf("smtp rcpt to: %w", err)
    }
    wc, err := c.Data()
    if err != nil {
        return fmt.Errorf("smtp data: %w", err)
    }
    _, err = wc.Write([]byte(msg))
    if err != nil {
        return fmt.Errorf("smtp write: %w", err)
    }
    if err := wc.Close(); err != nil {
        return fmt.Errorf("smtp close: %w", err)
    }
    return c.Quit()
}
