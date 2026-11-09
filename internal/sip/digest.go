package sip

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// ParseDigest 解析 Authorization 头里的 Digest 参数。
//
//	Digest username="340...", realm="3402000000", nonce="abc", uri="sip:...", response="..."
//	-> map[string]string{"username": "...", "realm": "...", ...}
//
// 把一长串 Authorization 头文本解析成 map
func ParseDigest(header string) map[string]string {
	result := make(map[string]string)

	// 去掉前缀 "Digest "
	if prefix, rest, ok := strings.Cut(strings.TrimSpace(header), " "); ok &&
		strings.EqualFold(prefix, "Digest") {
		header = rest
	}

	// 按逗号切分，但引号里的逗号不算分隔符。
	var fields []string
	var cur strings.Builder
	inQuote := false
	for _, r := range header {
		switch r {
		case '"':
			inQuote = !inQuote
			cur.WriteRune(r)
		case ',':
			if inQuote {
				cur.WriteRune(r)
			} else {
				fields = append(fields, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		fields = append(fields, cur.String())
	}

	for _, field := range fields {
		name, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.Trim(strings.TrimSpace(value), `"`)
		result[name] = value
	}
	return result
}

// VerifyDigest 校验设备发来的 Authorization。
//
// GB28181 一般不带 qop，公式是：
//
//	HA1      = MD5(username:realm:password)
//	HA2      = MD5(method:uri)
//	response = MD5(HA1:nonce:HA2)
//
// 如果设备带了 qop=auth，按 RFC 2617 的扩展公式校验。
func VerifyDigest(authorization, method, password string) bool {
	p := ParseDigest(authorization)

	username := p["username"]
	realm := p["realm"]
	nonce := p["nonce"]
	uri := p["uri"]
	response := p["response"]
	if response == "" || username == "" {
		return false
	}

	ha1 := md5Hex(username + ":" + realm + ":" + password)
	ha2 := md5Hex(method + ":" + uri)

	var expected string
	if qop := p["qop"]; qop != "" {
		// response = MD5(HA1:nonce:nc:cnonce:qop:HA2)
		expected = md5Hex(ha1 + ":" + nonce + ":" + p["nc"] + ":" +
			p["cnonce"] + ":" + qop + ":" + ha2)
	} else {
		expected = md5Hex(ha1 + ":" + nonce + ":" + ha2)
	}

	// 常量时间比较，避免通过响应时间猜出正确摘要。
	return subtle.ConstantTimeCompare(
		[]byte(strings.ToLower(expected)),
		[]byte(strings.ToLower(response)),
	) == 1
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
