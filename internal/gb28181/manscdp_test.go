package gb28181

import "testing"

func TestParseKeepalive(t *testing.T) {
	body := "<?xml version=\"1.0\" encoding=\"GB2312\"?>\r\n" +
		"<Notify>\r\n" +
		"  <CmdType>Keepalive</CmdType>\r\n" +
		"  <SN>42</SN>\r\n" +
		"  <DeviceID>34020000001320000001</DeviceID>\r\n" +
		"  <Status>OK</Status>\r\n" +
		"</Notify>\r\n"

	n, err := ParseNotify(body)
	if err != nil {
		t.Fatalf("ParseNotify: %v", err)
	}
	if n.CmdType != "Keepalive" {
		t.Errorf("CmdType = %q", n.CmdType)
	}
	if n.SN != 42 {
		t.Errorf("SN = %d", n.SN)
	}
	if n.DeviceID != "34020000001320000001" {
		t.Errorf("DeviceID = %q", n.DeviceID)
	}
	if n.Status != "OK" {
		t.Errorf("Status = %q", n.Status)
	}
}

func TestParseNotifyWithoutDeclaration(t *testing.T) {
	body := "<Notify><CmdType>Keepalive</CmdType><SN>1</SN><DeviceID>dev</DeviceID><Status>OK</Status></Notify>"

	n, err := ParseNotify(body)
	if err != nil {
		t.Fatalf("ParseNotify: %v", err)
	}
	if n.DeviceID != "dev" {
		t.Errorf("DeviceID = %q", n.DeviceID)
	}
}

func TestParseNotifyErrors(t *testing.T) {
	cases := []string{
		"",                            // 空
		"not xml at all",              // 不是 XML
		"<Notify><SN>1</SN></Notify>", // 缺 CmdType
	}
	for _, body := range cases {
		if _, err := ParseNotify(body); err == nil {
			t.Errorf("ParseNotify(%q) should fail", body)
		}
	}
}
