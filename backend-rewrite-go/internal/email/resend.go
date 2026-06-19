package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// resendClient posts to the Resend HTTP API (no SDK). Satisfies `sender`.
type resendClient struct{ http *http.Client }

func newResendClient() *resendClient { return &resendClient{http: &http.Client{}} }

func (c *resendClient) send(ctx context.Context, apiKey, from, to, subject, html string) (string, error) {
	body, _ := json.Marshal(map[string]string{"from": from, "to": to, "subject": subject, "html": html})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Data *struct {
			ID string `json:"id"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode >= 400 || out.Error != nil {
		msg := "send failed"
		if out.Error != nil {
			msg = out.Error.Message
		}
		return "", fmt.Errorf("%s", msg)
	}
	if out.Data != nil {
		return out.Data.ID, nil
	}
	return "", nil
}
