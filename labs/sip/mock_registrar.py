#!/usr/bin/env python3
"""
Task 2 学习用：一个最小的 GB28181 "假平台"（SIP 注册服务器）。

它只做三件事，够你看清 REGISTER 的完整往返：
  1. 收到不带 Authorization 的 REGISTER  -> 回 401 Unauthorized + nonce（挑战）
  2. 收到带 Authorization 的 REGISTER    -> 回 200 OK（注册成功）
  3. 收到 MESSAGE（保活）                -> 回 200 OK

运行（-u 让输出实时刷新，否则要等缓冲区满才打印）：
    python -u labs/sip/mock_registrar.py
"""
import re
import socket
import uuid

LISTEN = ("127.0.0.1", 5060)
REALM = "3402000000"          # GB28181 里通常是 SIP 域（设备 ID 的前 10 位）
PASSWORD = "12345678"         # 学习用，真实设备各自有密码

# SIP 是文本协议：一行一个头，头名: 值，\r\n 换行，空行之后是消息体。
def header(msg, name):
    """从头里取一个字段，比如 header(msg, 'Call-ID')。"""
    m = re.search(rf"^{name}:\s*(.*)$", msg, re.M | re.I)
    return m.group(1).strip() if m else ""


def build_response(req, code, reason, extra_headers=""):
    """
    构造 SIP 响应。响应必须原样带回请求里的 Via/From/Call-ID/CSeq，
    这是 SIP 能在 UDP 上匹配"这个响应属于哪个请求"的办法。
    """
    to = header(req, "To")
    if "tag=" not in to.lower():          # 响应里的 To 要加 tag（对话标识）
        to += ";tag=" + uuid.uuid4().hex[:8]

    lines = [
        f"SIP/2.0 {code} {reason}",         # 状态行：协议 状态码 原因
        f"Via: {header(req, 'Via')}",
        f"From: {header(req, 'From')}",
        f"To: {to}",
        f"Call-ID: {header(req, 'Call-ID')}",
        f"CSeq: {header(req, 'CSeq')}",
    ]
    if extra_headers:
        lines.extend(extra_headers.rstrip("\r\n").split("\r\n"))
    lines.append("Content-Length: 0")       # 没有消息体
    return "\r\n".join(lines) + "\r\n\r\n"  # 结尾空行 = 头结束


def main():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)  # UDP
    sock.bind(LISTEN)
    print(f"[platform] listening on udp {LISTEN[0]}:{LISTEN[1]}  realm={REALM}")
    print("[platform] Ctrl+C 退出\n")

    while True:
        data, addr = sock.recvfrom(65535)
        msg = data.decode("utf-8", "replace")
        first = msg.split("\r\n", 1)[0]     # 第一行是请求行

        print("=" * 70)
        print(f"[platform] <- {addr[0]}:{addr[1]}  {first}")
        print("-" * 70)
        print(msg)

        if first.startswith("REGISTER"):
            if header(msg, "Authorization"):
                # 第二次 REGISTER，带着 MD5 摘要：学习用，这里不校验，直接放行
                expires = header(msg, "Expires") or "3600"
                contact = header(msg, "Contact")
                resp = build_response(
                    msg, 200, "OK",
                    f"Expires: {expires}\r\nContact: {contact}",
                )
            else:
                # 第一次 REGISTER：发挑战，nonce 每次随机
                nonce = uuid.uuid4().hex[:16]
                resp = build_response(
                    msg, 401, "Unauthorized",
                    f'WWW-Authenticate: Digest realm="{REALM}", nonce="{nonce}"',
                )
        else:
            # MESSAGE（保活/目录查询）、INVITE 等，Task 2 先统一回 200
            resp = build_response(msg, 200, "OK")

        sock.sendto(resp.encode(), addr)
        print("-" * 70)
        print(f"[platform] -> {resp.split(chr(13) + chr(10))[0]}")


if __name__ == "__main__":
    main()
