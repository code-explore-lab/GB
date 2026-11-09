package sip

import (
	"strings"
	"testing"
)

func TestBuildResponseCopiesTransactionHeaders(t *testing.T) {
	req := Parse(strings.Join([]string{
		"REGISTER sip:x SIP/2.0",
		"Via: SIP/2.0/UDP 127.0.0.1:5099;branch=z9hG4bK123",
		"From: <sip:34020000001320000001@3402000000>;tag=abc",
		"To: <sip:34020000001320000001@3402000000>",
		"Call-ID: cid-123",
		"CSeq: 1 REGISTER",
		"Content-Length: 0",
	}, "\r\n") + "\r\n\r\n")

	resp := BuildResponse(req, 401, `WWW-Authenticate: Digest realm="3402000000", nonce="n"`)

	for _, want := range []string{
		"SIP/2.0 401 Unauthorized",
		"Via: SIP/2.0/UDP 127.0.0.1:5099;branch=z9hG4bK123",
		"From: <sip:34020000001320000001@3402000000>;tag=abc",
		"Call-ID: cid-123",
		"CSeq: 1 REGISTER",
		`WWW-Authenticate: Digest realm="3402000000", nonce="n"`,
		"Content-Length: 0",
	} {
		if !strings.Contains(resp, want) {
			t.Errorf("response missing %q:\n%s", want, resp)
		}
	}

	// 响应里的 To 必须带 tag。
	to := ""
	for line := range strings.SplitSeq(resp, "\r\n") {
		if strings.HasPrefix(line, "To: ") {
			to = line
			break
		}
	}
	if !strings.Contains(to, "tag=") {
		t.Errorf("To header missing tag: %q", to)
	}

	if !strings.HasSuffix(resp, "\r\n\r\n") {
		t.Errorf("response must end with a blank line: %q", resp)
	}
}

func TestURIUser(t *testing.T) {
	cases := map[string]string{
		"<sip:34020000001320000001@3402000000>;tag=abc": "34020000001320000001",
		"sip:34020000001320000001@3402000000":           "34020000001320000001",
		"<sip:34020000001320000001@3402000000>":         "34020000001320000001",
		"garbage":                                       "",
	}
	for in, want := range cases {
		if got := URIUser(in); got != want {
			t.Errorf("URIUser(%q) = %q, want %q", in, got, want)
		}
	}
}
