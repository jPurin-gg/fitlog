package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeOptionalJSON(t *testing.T) {
	for _, test := range []struct {
		body string
		want string
	}{
		{"", "unset"}, {"{}", "unset"}, {`{"use_ai":false}`, "off"}, {`{"use_ai":true}`, "on"},
		{`{"use_ai":"false"}`, "error"}, {`{"extra":true}`, "error"}, {"{", "error"},
		{"{} {}", "error"}, {strings.Repeat(" ", maxJSONBodyBytes+1), "error"},
	} {
		t.Run(test.want+"/"+test.body[:min(len(test.body), 40)], func(t *testing.T) {
			var input struct {
				UseAI *bool `json:"use_ai"`
			}
			r := httptest.NewRequest("POST", "/start", strings.NewReader(test.body))
			// Chunked bodies do not have a known Content-Length.
			r.ContentLength = -1
			err := DecodeOptionalJSON(httptest.NewRecorder(), r, &input)
			got := "unset"
			if err != nil {
				got = "error"
			} else if input.UseAI != nil {
				got = "off"
				if *input.UseAI {
					got = "on"
				}
			}
			if got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}
