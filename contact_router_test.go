package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoute(t *testing.T) {
	for _, tc := range []struct{ event, want string }{
		{"payment_question", "Payment question"},
		{"payment_dispute", "Payment dispute / review now"},
		{"security_report", "Security report / review now"},
	} {
		t.Run(tc.event, func(t *testing.T) {
			subject, body, err := route(submission{ID: "case-42", Email: "a@example.org", Name: "Ada", Message: "Please review", WidgetRecordID: "widget-42", CaptchaToken: "token", Event: tc.event, Reference: "pay-42"})
			if err != nil || !strings.Contains(subject, tc.want) || !strings.Contains(body, "pay-42") {
				t.Fatalf("subject=%q body=%q err=%v", subject, body, err)
			}
		})
	}
}

func TestCaptchaRejectionStopsEmail(t *testing.T) {
	emails := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/email/send" {
			emails++
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": map[string]string{"code": "CAPTCHA_SCORE_TOO_LOW"}})
	}))
	defer server.Close()
	h := contactHandler(infraiClient{baseURL: server.URL, key: "test", http: server.Client()}, "team@example.org")
	r := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(`{"id":"case-42","email":"a@example.org","name":"Ada","event":"payment_dispute","message":"Help","widget_record_id":"widget-42","captcha_token":"token"}`))
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusUnprocessableEntity || emails != 0 {
		t.Fatalf("status=%d emails=%d", w.Code, emails)
	}
}

func TestCapabilityPayloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		switch r.URL.Path {
		case "/v1/captcha/verify":
			if payload["widget_record_id"] != "widget-42" || payload["token"] != "token" {
				t.Fatalf("captcha payload=%v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]any{"verified": true}})
		case "/v1/email/send":
			if payload["body"] == nil || payload["text"] != nil {
				t.Fatalf("email payload=%v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]string{"message_id": "message-42"}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	h := contactHandler(infraiClient{baseURL: server.URL, key: "test", http: server.Client()}, "team@example.org")
	r := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(`{"id":"case-42","email":"a@example.org","name":"Ada","event":"payment_dispute","message":"Help","widget_record_id":"widget-42","captcha_token":"token"}`))
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
