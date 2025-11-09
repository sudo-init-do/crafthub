package alerts

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
)

type plunkConfig struct {
    APIKey string
    From   string
    APIURL string
}

var plunkCfg plunkConfig

// ConfigurePlunkFromEnv loads Plunk config from environment
// Required: PLUNK_API_KEY; Optional: PLUNK_FROM, PLUNK_API_URL
func ConfigurePlunkFromEnv() error {
    plunkCfg = plunkConfig{
        APIKey: os.Getenv("PLUNK_API_KEY"),
        From:   os.Getenv("PLUNK_FROM"),
        APIURL: os.Getenv("PLUNK_API_URL"),
    }
    if plunkCfg.APIURL == "" {
        plunkCfg.APIURL = "https://api.useplunk.com/v1/send"
    }
    if plunkCfg.APIKey == "" {
        return fmt.Errorf("plunk not configured: set PLUNK_API_KEY")
    }
    return nil
}

type plunkSendBody struct {
    To       string            `json:"to"`
    Subject  string            `json:"subject"`
    Body     string            `json:"body"`
    From     string            `json:"from,omitempty"`
    Headers  map[string]string `json:"headers,omitempty"`
    Subscribed bool            `json:"subscribed,omitempty"`
    Name     string            `json:"name,omitempty"`
    Reply    string            `json:"reply,omitempty"`
}

// sendViaPlunk performs the HTTP request to Plunk API
func sendViaPlunk(to, subject, body string) error {
    if plunkCfg.APIKey == "" {
        if err := ConfigurePlunkFromEnv(); err != nil {
            return err
        }
    }

    payload := plunkSendBody{
        To:      to,
        Subject: subject,
        Body:    body,
        From:    plunkCfg.From,
        Reply:   os.Getenv("MAIL_REPLY_TO"),
    }
    b, _ := json.Marshal(payload)
    req, err := http.NewRequest("POST", plunkCfg.APIURL, bytes.NewReader(b))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+plunkCfg.APIKey)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        // Try to read response body for more context
        var errMsg string
        if resp.Body != nil {
            if b, readErr := io.ReadAll(resp.Body); readErr == nil {
                if len(b) > 0 {
                    errMsg = string(b)
                }
            }
        }
        if errMsg != "" {
            return fmt.Errorf("plunk send failed: status=%d body=%s", resp.StatusCode, errMsg)
        }
        return fmt.Errorf("plunk send failed: status=%d", resp.StatusCode)
    }
    return nil
}
