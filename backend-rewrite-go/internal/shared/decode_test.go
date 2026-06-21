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
