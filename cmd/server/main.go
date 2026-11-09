// Task 5：SIP 服务器 —— REGISTER + 401 Digest 鉴权。
//
// 收到的请求在这里分发：
//
//	REGISTER -> 无 Authorization 回 401；带合法摘要回 200，并写入设备表
//	MESSAGE  -> 先统一回 200（Task 6 再处理保活内容）
//	其他     -> 405 Method Not Allowed
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"GB/internal/gb28181"
	"GB/internal/registry"
	"GB/internal/sip"
)

const (
	// 0.0.0.0 表示监听本机所有网卡。
	// 以后真实设备从局域网连进来，必须用 0.0.0.0，不能是 127.0.0.1。
	listenAddr = "0.0.0.0:5060"

	// SIP 域，GB28181 里通常是平台 ID 的前 10 位。
	realm = "3402000000"

	// 学习用的默认密码；真实环境必须按设备配置。
	defaultPassword = "12345678"
)

// 每个设备自己的密码。设备 ID -> 密码。
// 查不到时退回 defaultPassword（方便你换 ID 做实验）。
var passwords = map[string]string{
	"34020000001320000001": "12345678",
}

var (
	// 超过这个时长没收到保活，就认为设备离线。
	// GB28181 设备一般每 60 秒保活一次，所以 90 秒是常见的容错值。
	offlineTimeout = flag.Duration("offline-timeout", 90*time.Second, "超过该时长没有保活则标记离线")
	// 后台巡检间隔，越短越及时，但也没必要比超时还密。
	scanInterval = flag.Duration("scan-interval", 5*time.Second, "后台巡检间隔")
)

func main() {
	flag.Parse()

	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		log.Fatalf("resolve %s: %v", listenAddr, err)
	}

	// UDP 是无连接的：一次 Listen，就能收所有人的包。
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", listenAddr, err)
	}
	defer conn.Close()

	devices := registry.New()
	nonces := newNonceStore()

	// 后台巡检：和收包循环并行跑，不阻塞 SIP 响应。
	go monitorOffline(devices, *scanInterval, *offlineTimeout)

	log.Printf("SIP server listening on udp %s (realm=%s)", listenAddr, realm)
	log.Printf("offline detection: timeout=%s scan_interval=%s", *offlineTimeout, *scanInterval)

	buf := make([]byte, 64*1024) // SIP 报文一般很小，64KB 足够
	for {
		//UDP 是无连接的：一次 ReadFromUDP读取一个完整的UDP datagram
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read: %v", err)
			continue // 单个包出错不要退出整个服务
		}
		msg := sip.Parse(string(buf[:n]))
		if msg.Method == "" {
			continue
		}
		printMessage(remote, msg)

		resp := handle(remote, msg, devices, nonces)
		if resp == "" {
			continue
		}
		if _, err := conn.WriteToUDP([]byte(resp), remote); err != nil {
			log.Printf("send to %s: %v", remote, err)
			continue
		}
		log.Printf("-> %s  %s", remote, firstLine(resp))
	}
}

// handle 根据方法分发请求，返回要发送的响应文本。
func handle(remote *net.UDPAddr, m *sip.Message, devices *registry.Registry, nonces *nonceStore) string {
	switch m.Method {
	case "REGISTER":
		return handleRegister(remote, m, devices, nonces)
	case "MESSAGE":
		return handleMessage(remote, m, devices)
	default:
		return sip.BuildResponse(m, 405, "Allow: REGISTER, MESSAGE", "Server: GB-Mini-SIP")
	}
}

func handleRegister(remote *net.UDPAddr, m *sip.Message, devices *registry.Registry, nonces *nonceStore) string {
	deviceID := extractDeviceIDFromFrom(m.Get("from"))
	if deviceID == "" {
		return sip.BuildResponse(m, 400, "Server: GB-Mini-SIP")
	}

	authorization := m.Get("authorization")

	// 第一轮：没有鉴权头，发挑战。
	if authorization == "" {
		log.Printf("REGISTER from %s: no Authorization, sending 401 challenge", deviceID)
		return challenge(m, nonces)
	}

	// 第二轮：先确认 nonce 是我们发的、且没过期。
	parsed := sip.ParseDigest(authorization)
	if !nonces.valid(parsed["nonce"]) {
		log.Printf("REGISTER from %s: unknown or expired nonce, re-challenging", deviceID)
		return challenge(m, nonces)
	}

	// Authorization 里的 username/realm 必须和这次注册对得上。
	// 只校验摘要是不够的：否则 A 设备可以拿 B 的用户名去注册成 A。
	if parsed["username"] != deviceID || parsed["realm"] != realm {
		log.Printf("REGISTER from %s: username/realm mismatch (username=%q realm=%q)",
			deviceID, parsed["username"], parsed["realm"])
		return sip.BuildResponse(m, 403, "Server: GB-Mini-SIP")
	}

	// 再用设备密码重算摘要并比对。
	password, ok := passwords[deviceID]
	if !ok {
		log.Printf("device %s has no configured password, using default", deviceID)
		password = defaultPassword
	}
	if !sip.VerifyDigest(authorization, "REGISTER", password) {
		log.Printf("REGISTER from %s: digest verification FAILED", deviceID)
		return sip.BuildResponse(m, 403, "Server: GB-Mini-SIP")
	}

	// 校验通过：写入设备表。
	expires := parseExpires(m.Get("expires"))
	now := time.Now()
	devices.Upsert(registry.Device{
		ID:        deviceID,
		Addr:      remote,
		ExpiresAt: now.Add(expires),
		LastSeen:  now,
		Online:    true,
	})
	log.Printf("device registered: %s  addr=%s  expires_in=%s", deviceID, remote, expires)
	dumpRegistry(devices)

	return sip.BuildResponse(m, 200,
		"Contact: "+m.Get("contact"),
		fmt.Sprintf("Expires: %d", int(expires.Seconds())),
		"Server: GB-Mini-SIP",
	)
}

// handleMessage 处理 MESSAGE：目前只关心 Keepalive。
func handleMessage(remote *net.UDPAddr, m *sip.Message, devices *registry.Registry) string {
	notify, err := gb28181.ParseNotify(m.Body)
	if err != nil {
		// 解析不了也回 200：SIP 层面确实收到了，
		// 回错误会让设备不断重发，反而放大问题。
		log.Printf("MESSAGE from %s: bad body: %v", remote, err)
		return sip.BuildResponse(m, 200, "Server: GB-Mini-SIP")
	}

	if !strings.EqualFold(notify.CmdType, "Keepalive") {
		log.Printf("MESSAGE CmdType=%s from %s not handled yet", notify.CmdType, remote)
		return sip.BuildResponse(m, 200, "Server: GB-Mini-SIP")
	}

	id := notify.DeviceID
	if id == "" {
		id = extractDeviceIDFromFrom(m.Get("from"))
	}

	if devices.Touch(id, remote, time.Now()) {
		log.Printf("keepalive: %s online (SN=%d status=%s)", id, notify.SN, notify.Status)
	} else {
		// 没注册过的设备也能发保活，忽略但不报错。
		log.Printf("keepalive from unregistered device %s, ignored", id)
	}

	return sip.BuildResponse(m, 200, "Server: GB-Mini-SIP")
}

// monitorOffline 是后台巡检协程：
// 每隔 interval 扫一次，把超过 timeout 没保活的设备标记离线。
func monitorOffline(devices *registry.Registry, interval, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for now := range ticker.C {
		for _, id := range devices.MarkStaleOffline(now, timeout) {
			log.Printf("device OFFLINE: %s (no keepalive for > %s)", id, timeout)
			dumpRegistry(devices)
		}
	}
}

// challenge 生成 401 + WWW-Authenticate。
func challenge(m *sip.Message, nonces *nonceStore) string {
	nonce := nonces.issue()
	return sip.BuildResponse(m, 401,
		fmt.Sprintf(`WWW-Authenticate: Digest realm="%s", nonce="%s", algorithm=MD5`, realm, nonce),
		"Server: GB-Mini-SIP",
	)
}

func parseExpires(v string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || seconds <= 0 {
		seconds = 3600
	}
	if seconds > 86400 {
		seconds = 86400 // 上限一天，防止设备报一个巨大的值
	}
	return time.Duration(seconds) * time.Second
}

func dumpRegistry(devices *registry.Registry) {
	log.Printf("registry (%d devices):", devices.Len())
	for _, d := range devices.All() {
		log.Printf("  %s  addr=%s  expires_at=%s  online=%v",
			d.ID, d.Addr, d.ExpiresAt.Format("15:04:05"), d.Online)
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\r\n")
	return line
}

// nonceStore 记住服务器发出去的 nonce，防止客户端自己编一个来通过校验。
type nonceStore struct {
	mu     sync.Mutex
	issued map[string]time.Time // nonce -> 过期时间
}

func newNonceStore() *nonceStore {
	return &nonceStore{issued: make(map[string]time.Time)}
}

func (s *nonceStore) issue() string {
	nonce := sip.NewNonce()
	s.mu.Lock()
	s.issued[nonce] = time.Now().Add(5 * time.Minute)
	s.mu.Unlock()
	return nonce
}

func (s *nonceStore) valid(nonce string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	expiresAt, ok := s.issued[nonce]
	if !ok {
		return false
	}
	if time.Now().After(expiresAt) {
		delete(s.issued, nonce)
		return false
	}
	return true
}

func extractDeviceIDFromFrom(from string) string {
	s := strings.TrimPrefix(from, "<sip:")
	idx := strings.Index(s, "@")
	if idx == -1 {
		return ""
	}
	return s[:idx]
}

func printMessage(remote *net.UDPAddr, m *sip.Message) {
	line := strings.Repeat("=", 70)
	fmt.Println(line)
	// 起始行拆出来的三个关键字段
	fromVal := m.Get("from")
	devID := extractDeviceIDFromFrom(fromVal)
	if devID != "" {
		fmt.Printf("设备 %s 发来 %s\n", devID, m.Method)
	}
	fmt.Printf("<<< from %s\n", remote)
	fmt.Printf("    method=%q uri=%q version=%q\n", m.Method, m.URI, m.Version)
	fmt.Println(strings.Repeat("-", 70))

	// 头字段排序后打印，输出稳定，方便对比
	names := make([]string, 0, len(m.Headers))
	for name := range m.Headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Printf("    %-18s %s\n", name+":", strings.Join(m.Headers[name], ", "))
	}

	// 保活 MESSAGE 的消息体是 XML，要能打出来
	fmt.Printf("\n    body (%d bytes):\n%s\n", len(m.Body), m.Body)

	// 原始文本最后打印：对照 Wireshark 时看这个
	fmt.Println("    --- raw ---")
	fmt.Print(m.Raw)
	fmt.Println()
}
