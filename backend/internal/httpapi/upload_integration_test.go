package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"testing"
	"time"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/upload"
)

func (h *harness) uploadFile(t *testing.T, token string, data []byte, field string, count int, extra bool) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for i := 0; i < count; i++ {
		part, err := writer.CreateFormFile(field, `..\..\report.html`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if extra {
		if err := writer.WriteField("name", "unexpected"); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/uploads", &body)
	r.RemoteAddr = "192.0.2.50:1234"
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.api.ServeHTTP(w, r)
	return w
}

func TestIntegrationUploads(t *testing.T) {
	h := integration(t)
	account, _ := result(t, h.do("POST", "register", credentials("files@example.com", testPassword), "", ""), 201)
	data := []byte("<html><script>alert(1)</script></html>")
	expect(t, h.uploadFile(t, "", data, "file", 1, false), 401)
	expect(t, h.uploadFile(t, "invalid", data, "file", 1, false), 401)
	for _, data := range [][]byte{data, []byte("%PDF-1.7\nexample"), {'P', 'K', 0, 255, 1}, {}} {
		w := h.uploadFile(t, account.AccessToken, data, "file", 1, false)
		expect(t, w, 201)
		var file upload.File
		if err := json.Unmarshal(w.Body.Bytes(), &file); err != nil || file.URL == "" || file.Name != "report.html" || file.Size != int64(len(data)) || file.ContentType == "" {
			t.Fatalf("file metadata: %s, %v", w.Body, err)
		}
		w = httptest.NewRecorder()
		h.api.ServeHTTP(w, httptest.NewRequest("GET", file.URL, nil))
		expect(t, w, 200)
		if !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Content-Type") != "application/octet-stream" || w.Header().Get("Content-Disposition") == "" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("download corrupted or inline", w.Header())
		}
	}
	h.clock.Add(int64(auth.AccessTTL))
	expect(t, h.uploadFile(t, account.AccessToken, data, "file", 1, false), 401)
}

func TestIntegrationUploadValidation(t *testing.T) {
	h := integration(t)
	account, _ := result(t, h.do("POST", "register", credentials("upload-validation@example.com", testPassword), "", ""), 201)
	for _, tc := range []struct {
		name  string
		data  []byte
		field string
		count int
		extra bool
		code  int
	}{
		{"missing", nil, "file", 0, false, 400},
		{"wrong field", []byte("test"), "avatar", 1, false, 400},
		{"duplicate", []byte("test"), "file", 2, false, 400},
		{"extra field", []byte("test"), "file", 1, true, 400},
		{"file limit", bytes.Repeat([]byte("x"), upload.MaxFileBytes+1), "file", 1, false, 413},
		{"body limit", bytes.Repeat([]byte("x"), upload.MaxFileBytes+(128<<10)), "file", 1, false, 413},
		{"exact limit", bytes.Repeat([]byte("x"), upload.MaxFileBytes), "file", 1, false, 201},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expect(t, h.uploadFile(t, account.AccessToken, tc.data, tc.field, tc.count, tc.extra), tc.code)
		})
	}
	// Rate limits and multipart guards also apply to the generic endpoint.
	h.clock.Add(int64(time.Minute))
	for i := 0; i < 11; i++ {
		w := h.uploadFile(t, account.AccessToken, []byte("test"), "file", 1, false)
		if i < 10 {
			expect(t, w, 201)
		} else {
			expect(t, w, 429)
		}
	}
}
