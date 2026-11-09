// Task 4：SIP UDP 服务器骨架。
//
// 现在只做两件事：
//  1. 监听 UDP 5060
//  2. 收到包 -> 解析 -> 打印
//
// Task 5 会在这里加"收到 REGISTER 回 401"的逻辑。
package main

import (
	"fmt"
	"log"
	"net"
	"sort"
	"strings"

	"GB/internal/sip"
)

// 0.0.0.0 表示监听本机所有网卡。
// 以后真实设备从局域网连进来，必须用 0.0.0.0，不能是 127.0.0.1。
const listenAddr = "0.0.0.0:5060"

func main() {
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

	log.Printf("SIP server listening on udp %s", listenAddr)

	buf := make([]byte, 64*1024) // SIP 报文一般很小，64KB 足够
	for {
		//UDP 是无连接的：一次 ReadFromUDP读取一个完整的UDP datagram
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read: %v", err)
			continue // 单个包出错不要退出整个服务
		}
		msg := sip.Parse(string(buf[:n]))
		printMessage(remote, msg)
	}
}

func printMessage(remote *net.UDPAddr, m *sip.Message) {
	line := strings.Repeat("=", 70)
	fmt.Println(line)
	// 起始行拆出来的三个关键字段
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
