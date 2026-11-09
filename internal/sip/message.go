// Package sip 负责解析 SIP 文本报文。
//
// SIP 报文的结构和 HTTP 几乎一样（RFC 3261）：
//
//	起始行\r\n          ← 请求: "REGISTER sip:xxx SIP/2.0"；响应: "SIP/2.0 200 OK"
//	头字段: 值\r\n
//	头字段: 值\r\n
//	\r\n                 ← 空行，表示头结束
//	消息体               ← 可以没有（Content-Length: 0）
package sip

import "strings"

// Message 是一条解析后的 SIP 报文。
type Message struct {
	StartLine string              // 原始起始行
	Method    string              // 请求方法，如 REGISTER / MESSAGE / INVITE（响应为空）
	URI       string              // 请求 URI（响应为空）
	Version   string              // 一般为 SIP/2.0
	Headers   map[string][]string // 头字段，key 统一转小写，value 可能有多个
	Body      string              // 消息体（保活的 XML 就在这里）
	Raw       string              // 原始文本，打印/排错用
}

// Parse 把一段 SIP 文本解析成 Message。
// 解析失败不会 panic，尽量保留能解析的部分。
func Parse(raw string) *Message {
	// 1. 头和体之间隔一个空行。
	//    头部统一成 \n 再按行切；body 必须保持原样，
	//    否则 \r 被吃掉，字节数和 Content-Length 对不上。
	var head, body string
	if h, b, ok := strings.Cut(raw, "\r\n\r\n"); ok {
		head, body = h, b
	} else if h, b, ok := strings.Cut(raw, "\n\n"); ok {
		head, body = h, b // 容忍只发 \n 的客户端
	} else {
		head = raw
	}
	head = strings.ReplaceAll(head, "\r\n", "\n")

	lines := strings.Split(head, "\n")
	if len(lines) == 0 {
		return &Message{Raw: raw, Headers: map[string][]string{}}
	}

	m := &Message{
		StartLine: lines[0],
		Headers:   make(map[string][]string),
		Body:      body,
		Raw:       raw,
	}

	// 2. 起始行：请求是 "方法 URI 版本"，响应是 "版本 状态码 原因"。
	parts := strings.SplitN(lines[0], " ", 3)
	if len(parts) == 3 {
		if strings.HasPrefix(parts[0], "SIP/2.0") {
			m.Version = parts[0]
		} else {
			m.Method, m.URI, m.Version = parts[0], parts[1], parts[2]
		}
	}

	// 3. 头字段：按第一个冒号切开，头名大小写不敏感（SIP 规定）。
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue // 不是合法头，跳过而不是报错
		}
		key := strings.ToLower(strings.TrimSpace(name))
		m.Headers[key] = append(m.Headers[key], strings.TrimSpace(value))
	}

	return m
}

// Get 取一个头的第一个值，头名大小写不敏感。
// Get("call-id") 和 Get("Call-ID") 等价。
func (m *Message) Get(name string) string {
	values := m.Headers[strings.ToLower(name)]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
