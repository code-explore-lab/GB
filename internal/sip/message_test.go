package sip

import (
	"strconv"
	"strings"
	"testing"
)

// 用 Task 2 抓到的真实 REGISTER 报文做测试数据。
func TestParseRegister(t *testing.T) {
	raw := strings.Join([]string{
		"REGISTER sip:34020000002000000001@3402000000 SIP/2.0",
		"Via: SIP/2.0/UDP 127.0.0.1:5099;rport;branch=z9hG4bK123",
		"From: <sip:34020000001320000001@3402000000>;tag=abc",
		"To: <sip:34020000001320000001@3402000000>",
		"Call-ID: 7948ee8f67f6413096e0569bfcf3437e",
		"CSeq: 1 REGISTER",
		"Contact: <sip:34020000001320000001@127.0.0.1:5099>",
		"Content-Length: 0",
	}, "\r\n") + "\r\n\r\n"

	m := Parse(raw)

	if m.Method != "REGISTER" {
		t.Errorf("Method = %q, want REGISTER", m.Method)
	}
	if m.URI != "sip:34020000002000000001@3402000000" {
		t.Errorf("URI = %q", m.URI)
	}
	if m.Version != "SIP/2.0" {
		t.Errorf("Version = %q", m.Version)
	}
	if got := m.Get("Call-ID"); got != "7948ee8f67f6413096e0569bfcf3437e" {
		t.Errorf("Call-ID = %q", got)
	}
	// 头名大小写不敏感
	if got := m.Get("call-id"); got != m.Get("Call-ID") {
		t.Errorf("lowercase lookup mismatch: %q vs %q", got, m.Get("Call-ID"))
	}
	if m.Body != "" {
		t.Errorf("Body = %q, want empty", m.Body)
	}
}

// body 里的 \r\n 必须原样保留，长度要和 Content-Length 一致。
func TestParseKeepaliveBodyPreserved(t *testing.T) {
	body := "<?xml version=\"1.0\" encoding=\"GB2312\"?>\r\n" +
		"<Notify>\r\n" +
		"  <CmdType>Keepalive</CmdType>\r\n" +
		"  <SN>1</SN>\r\n" +
		"  <DeviceID>34020000001320000001</DeviceID>\r\n" +
		"  <Status>OK</Status>\r\n" +
		"</Notify>\r\n"

	raw := strings.Join([]string{
		"MESSAGE sip:34020000002000000001@3402000000 SIP/2.0",
		"Via: SIP/2.0/UDP 127.0.0.1:5099;branch=z9hG4bK456",
		"Call-ID: 0e583625250145ef85d9f8e5461ea9f8",
		"CSeq: 2 MESSAGE",
		"Content-Type: Application/MANSCDP+xml",
		"Content-Length: " + strconv.Itoa(len(body)),
	}, "\r\n") + "\r\n\r\n" + body

	m := Parse(raw)

	if m.Body != body {
		t.Errorf("body not preserved:\n got %q\nwant %q", m.Body, body)
	}
	if len(m.Body) != len(body) {
		t.Errorf("body length = %d, want %d", len(m.Body), len(body))
	}
	if got, want := m.Get("content-length"), strconv.Itoa(len(m.Body)); got != want {
		t.Errorf("Content-Length = %s, body = %s bytes", got, want)
	}
	if !strings.Contains(m.Body, "<CmdType>Keepalive</CmdType>") {
		t.Errorf("keepalive XML missing: %q", m.Body)
	}
}

// 响应的起始行是 "SIP/2.0 401 Unauthorized"，没有方法名。
func TestParseResponse(t *testing.T) {
	raw := "SIP/2.0 401 Unauthorized\r\n" +
		"Via: SIP/2.0/UDP 127.0.0.1:5099;branch=z9hG4bK789\r\n" +
		"WWW-Authenticate: Digest realm=\"3402000000\", nonce=\"abc\"\r\n" +
		"Content-Length: 0\r\n\r\n"

	m := Parse(raw)

	if m.Method != "" {
		t.Errorf("Method = %q, want empty for response", m.Method)
	}
	if m.Version != "SIP/2.0" {
		t.Errorf("Version = %q", m.Version)
	}
	if got := m.Get("www-authenticate"); !strings.Contains(got, "nonce=") {
		t.Errorf("WWW-Authenticate = %q", got)
	}
}

// 容忍只用 \n 换行的客户端。
func TestParseLFOnly(t *testing.T) {
	raw := "REGISTER sip:a@b SIP/2.0\nCall-ID: xyz\nContent-Length: 0\n\n"

	m := Parse(raw)

	if m.Method != "REGISTER" {
		t.Errorf("Method = %q", m.Method)
	}
	if got := m.Get("call-id"); got != "xyz" {
		t.Errorf("Call-ID = %q", got)
	}
}
