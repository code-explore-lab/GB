package sip

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// 常用状态码及原因短语。
var reasons = map[int]string{
	200: "OK",
	400: "Bad Request",
	401: "Unauthorized",
	403: "Forbidden",
	404: "Not Found",
	405: "Method Not Allowed",
	500: "Server Internal Error",
}

// NewNonce 生成一个随机的 nonce（挑战值）。
func NewNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 极端情况下退化成时间戳，至少不会返回空串
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// RandomTag 生成 To 头用的对话 tag。
func RandomTag() string {
	return NewNonce()[:8]
}

// BuildResponse 根据请求构造响应。
//
// SIP 响应必须把 Via / From / Call-ID / CSeq 原样带回，
// 客户端靠这些字段匹配"这个响应属于哪个请求"。
// extraHeaders 是附加头，例如 WWW-Authenticate、Contact、Expires。
func BuildResponse(req *Message, code int, extraHeaders ...string) string {
	reason, ok := reasons[code]
	if !ok {
		reason = "Unknown"
	}

	to := req.Get("to")
	if !strings.Contains(strings.ToLower(to), "tag=") {
		to += ";tag=" + RandomTag()
	}

	lines := []string{
		fmt.Sprintf("SIP/2.0 %d %s", code, reason),
		"Via: " + req.Get("via"),
		"From: " + req.Get("from"),
		"To: " + to,
		"Call-ID: " + req.Get("call-id"),
		"CSeq: " + req.Get("cseq"),
	}
	lines = append(lines, extraHeaders...)
	lines = append(lines, "Content-Length: 0")

	return strings.Join(lines, "\r\n") + "\r\n\r\n"
}

// URIUser 从 From/To 头里取出用户部分（设备 ID）。
//
//	<sip:34020000001320000001@3402000000>;tag=abc  ->  34020000001320000001
func URIUser(v string) string {
	i := strings.Index(strings.ToLower(v), "sip:")
	if i < 0 {
		return ""
	}
	rest := v[i+4:]
	if j := strings.IndexAny(rest, "@>;"); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}
