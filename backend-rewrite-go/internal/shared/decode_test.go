package shared

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecodeJSON(t *testing.T) {
	type body struct {
		Email string `json:"email"`
	}

	const nullMsg = `Unexpected token 'n', "null" is not valid JSON`
	const badMsg = `Invalid request body`

	cases := []struct {
		name string
		in   string
		mode DecodeMode

		wantOK       bool
		wantStatus   int    // only checked when wantOK == false
		wantErrMsg   string // only checked when wantOK == false
		wantEmail    string // only checked when wantOK == true
		wantEmailSet bool   // assert dst.Email when wantOK == true
	}{
		// literal null — rejected in ANY mode.
		{name: "null/strict", in: "null", mode: DecodeStrict, wantOK: false, wantStatus: 400, wantErrMsg: nullMsg},
		{name: "null/optional", in: "null", mode: DecodeOptional, wantOK: false, wantStatus: 400, wantErrMsg: nullMsg},
		{name: "null/lenient", in: "null", mode: DecodeLenient, wantOK: false, wantStatus: 400, wantErrMsg: nullMsg},

		// whitespace-wrapped null — rejected in ANY mode.
		{name: "wsnull/strict", in: "  null  ", mode: DecodeStrict, wantOK: false, wantStatus: 400, wantErrMsg: nullMsg},
		{name: "wsnull/optional", in: "  null  ", mode: DecodeOptional, wantOK: false, wantStatus: 400, wantErrMsg: nullMsg},
		{name: "wsnull/lenient", in: "  null  ", mode: DecodeLenient, wantOK: false, wantStatus: 400, wantErrMsg: nullMsg},

		// empty body.
		{name: "empty/strict", in: "", mode: DecodeStrict, wantOK: false, wantStatus: 400, wantErrMsg: badMsg},
		{name: "empty/optional", in: "", mode: DecodeOptional, wantOK: true, wantEmail: "", wantEmailSet: true},
		{name: "empty/lenient", in: "", mode: DecodeLenient, wantOK: true},

		// malformed body.
		{name: "malformed/strict", in: "{", mode: DecodeStrict, wantOK: false, wantStatus: 400, wantErrMsg: badMsg},
		{name: "malformed/optional", in: "{", mode: DecodeOptional, wantOK: false, wantStatus: 400, wantErrMsg: badMsg},
		{name: "malformed/lenient", in: "{", mode: DecodeLenient, wantOK: true},

		// valid body.
		{name: "valid/strict", in: `{"email":"a"}`, mode: DecodeStrict, wantOK: true, wantEmail: "a", wantEmailSet: true},
		{name: "valid/optional", in: `{"email":"a"}`, mode: DecodeOptional, wantOK: true, wantEmail: "a", wantEmailSet: true},
		{name: "valid/lenient", in: `{"email":"a"}`, mode: DecodeLenient, wantOK: true, wantEmail: "a", wantEmailSet: true},

		// valid with trailing data — Decoder stops after first value.
		{name: "trailing/strict", in: `{}{}`, mode: DecodeStrict, wantOK: true},
		{name: "trailing/optional", in: `{}{}`, mode: DecodeOptional, wantOK: true},
		{name: "trailing/lenient", in: `{}{}`, mode: DecodeLenient, wantOK: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(tc.in)))
			var dst body

			got := DecodeJSON(rec, req, &dst, tc.mode)

			if got != tc.wantOK {
				t.Fatalf("DecodeJSON returned %v, want %v", got, tc.wantOK)
			}

			if !tc.wantOK {
				if rec.Code != tc.wantStatus {
					t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
				}
				var env struct {
					Error string `json:"error"`
					Code  string `json:"code"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
					t.Fatalf("response body not JSON: %v (body=%q)", err, rec.Body.String())
				}
				if env.Error != tc.wantErrMsg {
					t.Fatalf("error = %q, want %q", env.Error, tc.wantErrMsg)
				}
				if env.Code != "invalid-input" {
					t.Fatalf("code = %q, want %q", env.Code, "invalid-input")
				}
				return
			}

			// wantOK == true
			if tc.wantEmailSet && dst.Email != tc.wantEmail {
				t.Fatalf("dst.Email = %q, want %q", dst.Email, tc.wantEmail)
			}
		})
	}
}

// TestDecodeJSON_RequestIDParity proves the body-parser-level parity rule:
// a literal-null rejection mirrors Node's empty request_id (Node's
// body-parser throws before the request-id middleware), while a malformed
// rejection inside the handler still carries the populated context id.
// Both requests run through the real RequestID middleware so the context
// id is non-empty — the difference is which envelope writer DecodeJSON uses.
func TestDecodeJSON_RequestIDParity(t *testing.T) {
	type body struct {
		Email string `json:"email"`
	}
	decodeID := func(reqBody string) (status int, requestID string) {
		var dst body
		h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			DecodeJSON(w, r, &dst, DecodeStrict)
		}))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(reqBody)))
		h.ServeHTTP(rec, req)
		var env struct {
			RequestID string `json:"request_id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		return rec.Code, env.RequestID
	}

	// null → body-parse rejection → empty request_id (Node parity).
	if status, id := decodeID("null"); status != 400 || id != "" {
		t.Fatalf("null body: got status=%d request_id=%q, want status=400 request_id=\"\"", status, id)
	}
	// malformed (handler-level) → populated request_id from context.
	if status, id := decodeID("{"); status != 400 || id == "" {
		t.Fatalf("malformed body: got status=%d request_id=%q, want status=400 non-empty request_id", status, id)
	}
}
