package sip

import "testing"

// GB28181 常见场景：不带 qop。
// 期望值用 Python 的 hashlib 独立算出来，不是用被测代码本身算的。
func TestVerifyDigestNoQop(t *testing.T) {
	auth := `Digest username="34020000001320000001", realm="3402000000", ` +
		`nonce="abc123", uri="sip:34020000002000000001@3402000000", ` +
		`response="200065e22cfd50de0c58a623ef9e91f8", algorithm=MD5`

	if !VerifyDigest(auth, "REGISTER", "12345678") {
		t.Fatal("valid digest rejected")
	}
	if VerifyDigest(auth, "REGISTER", "wrong-password") {
		t.Fatal("wrong password accepted")
	}
	if VerifyDigest(auth, "MESSAGE", "12345678") {
		t.Fatal("digest accepted with wrong method")
	}
}

// RFC 2617 的官方示例，带 qop=auth，用来验证扩展公式。
func TestVerifyDigestWithQop(t *testing.T) {
	auth := `Digest username="Mufasa", realm="testrealm@host.com", ` +
		`nonce="dcd98b7102dd2f0e8b11d0f600bfb0c093", uri="/dir/index.html", ` +
		`qop=auth, nc=00000001, cnonce="0a4f113b", ` +
		`response="6629fae49393a05397450978507c4ef1", ` +
		`opaque="5ccc069c403ebaf9f0171e9517f40e41"`

	if !VerifyDigest(auth, "GET", "Circle Of Life") {
		t.Fatal("RFC 2617 example rejected")
	}
}

func TestParseDigest(t *testing.T) {
	auth := `Digest username="dev", realm="r", nonce="n", uri="sip:x", response="abc", algorithm=MD5`
	p := ParseDigest(auth)

	if p["username"] != "dev" || p["realm"] != "r" || p["nonce"] != "n" {
		t.Fatalf("unexpected parse result: %#v", p)
	}
	if p["response"] != "abc" || p["algorithm"] != "MD5" {
		t.Fatalf("unexpected parse result: %#v", p)
	}
}
