package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type submission struct {
	ID             string `json:"id"`
	Email          string `json:"email"`
	Name           string `json:"name"`
	Event          string `json:"event"`
	Reference      string `json:"reference"`
	Message        string `json:"message"`
	WidgetRecordID string `json:"widget_record_id"`
	CaptchaToken   string `json:"captcha_token"`
}

type apiError struct {
	Code string `json:"code"`
	Hint string `json:"hint"`
}

func (e *apiError) Error() string { return e.Code + ": " + e.Hint }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type infraiClient struct {
	baseURL, key string
	http         *http.Client
}

// call reads the Infrai envelope before interpreting an HTTP rejection.
func (c infraiClient) call(ctx context.Context, path string, payload any, id string, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", id)
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		var env envelope
		err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&env)
		res.Body.Close()
		if err != nil {
			return fmt.Errorf("decode Infrai response: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds >= 0 && seconds <= 30 {
				delay = time.Duration(seconds) * time.Second
			}
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if !env.OK {
			if env.Error != nil {
				return env.Error
			}
			return fmt.Errorf("Infrai request rejected (HTTP %d)", res.StatusCode)
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai HTTP %d", res.StatusCode)
		}
		return json.Unmarshal(env.Data, result)
	}
	return errors.New("retry limit reached")
}

func route(s submission) (string, string, error) {
	if strings.TrimSpace(s.ID) == "" || !strings.Contains(s.Email, "@") || strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.Message) == "" || strings.TrimSpace(s.WidgetRecordID) == "" || strings.TrimSpace(s.CaptchaToken) == "" {
		return "", "", errors.New("id, email, name, message, widget_record_id and captcha_token are required")
	}
	var label string
	switch s.Event {
	case "payment_question":
		label = "Payment question"
	case "payment_dispute":
		label = "Payment dispute / review now"
	case "security_report":
		label = "Security report / review now"
	default:
		return "", "", errors.New("event must be payment_question, payment_dispute or security_report")
	}
	clean := func(v string) string {
		return strings.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r < 32 {
				return ' '
			}
			return r
		}, v)
	}
	subject := fmt.Sprintf("[%s] %s", label, clean(s.ID))
	text := fmt.Sprintf("Event: %s\nSubmission: %s\nReference: %s\nContact: %s <%s>\n\n%s", s.Event, clean(s.ID), clean(s.Reference), clean(s.Name), clean(s.Email), s.Message)
	return subject, text, nil
}

func stableID(prefix, id string) string {
	sum := sha256.Sum256([]byte(prefix + ":" + id))
	return prefix + "-" + hex.EncodeToString(sum[:])
}

func contactHandler(c infraiClient, inbox string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var s submission
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&s); err != nil {
			http.Error(w, "invalid submission", http.StatusBadRequest)
			return
		}
		subject, body, err := route(s)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var verified json.RawMessage
		err = c.call(r.Context(), "/v1/captcha/verify", map[string]any{"widget_record_id": s.WidgetRecordID, "token": s.CaptchaToken}, stableID("captcha", s.ID), &verified)
		if err != nil {
			upstreamError(w, err)
			return
		}
		var sent struct {
			MessageID string `json:"message_id"`
		}
		err = c.call(r.Context(), "/v1/email/send", map[string]any{"to": inbox, "subject": subject, "body": body}, stableID("email", s.ID), &sent)
		if err != nil {
			upstreamError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"message_id": sent.MessageID})
	}
}

func upstreamError(w http.ResponseWriter, err error) {
	var rejected *apiError
	if errors.As(err, &rejected) {
		http.Error(w, "submission rejected: "+rejected.Code, http.StatusUnprocessableEntity)
		return
	}
	http.Error(w, "delivery unavailable", http.StatusBadGateway)
}
