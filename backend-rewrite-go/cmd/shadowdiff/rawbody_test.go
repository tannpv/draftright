package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSubstituteRawBody confirms {{token}} placeholders are replaced inside
// raw_body (multipart payloads may carry a bearer field).
func TestSubstituteRawBody(t *testing.T) {
	f := fixture{RawBody: "name={{user_token}}&x=1"}
	got := substitute(f, map[string]string{"user_token": "TKN"})
	if got.RawBody != "name=TKN&x=1" {
		t.Fatalf("raw_body not substituted: %q", got.RawBody)
	}
	if f.RawBody != "name={{user_token}}&x=1" {
		t.Fatalf("input fixture mutated: %q", f.RawBody)
	}
}

// TestSendRawBodyVerbatim confirms send streams raw_body byte-for-byte (CRLF
// preserved) and that raw_body takes precedence over body.
func TestSendRawBodyVerbatim(t *testing.T) {
	want := "--b\r\nContent-Disposition: form-data; name=\"website\"\r\n\r\nspam\r\n--b--\r\n"
	var got string
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(201)
	}))
	defer srv.Close()

	f := fixture{
		Method:  "POST",
		Path:    "/bug-reports",
		Headers: map[string]string{"Content-Type": "multipart/form-data; boundary=b"},
		Body:    []byte(`{"ignored":true}`),
		RawBody: want,
	}
	status, _, _, err := send(srv.Client(), srv.URL, f)
	if err != nil {
		t.Fatal(err)
	}
	if status != 201 {
		t.Fatalf("status = %d", status)
	}
	if got != want {
		t.Fatalf("raw_body not sent verbatim:\n got %q\nwant %q", got, want)
	}
	if gotCT != "multipart/form-data; boundary=b" {
		t.Fatalf("content-type = %q", gotCT)
	}
}
