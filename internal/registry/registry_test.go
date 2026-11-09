package registry

import (
	"net"
	"testing"
	"time"
)

func TestUpsertAndGet(t *testing.T) {
	r := New()
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.10"), Port: 5060}
	d := Device{
		ID:        "34020000001320000001",
		Addr:      addr,
		ExpiresAt: time.Now().Add(time.Hour),
		LastSeen:  time.Now(),
		Online:    true,
	}

	r.Upsert(d)

	got, ok := r.Get(d.ID)
	if !ok {
		t.Fatal("device not found")
	}
	if got.Addr.String() != "192.168.1.10:5060" {
		t.Errorf("Addr = %s", got.Addr)
	}
	if r.Len() != 1 {
		t.Errorf("Len = %d, want 1", r.Len())
	}
}

func TestUpsertOverwritesSameDevice(t *testing.T) {
	r := New()
	id := "34020000001320000001"

	r.Upsert(Device{ID: id, Addr: &net.UDPAddr{Port: 5060}})
	r.Upsert(Device{ID: id, Addr: &net.UDPAddr{Port: 5070}})

	if r.Len() != 1 {
		t.Fatalf("Len = %d, want 1 (same device must overwrite)", r.Len())
	}
	got, _ := r.Get(id)
	if got.Addr.Port != 5070 {
		t.Errorf("Port = %d, want 5070", got.Addr.Port)
	}
}

func TestAllReturnsSortedSnapshot(t *testing.T) {
	r := New()
	r.Upsert(Device{ID: "b"})
	r.Upsert(Device{ID: "a"})
	r.Upsert(Device{ID: "c"})

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("len = %d, want 3", len(all))
	}
	if all[0].ID != "a" || all[1].ID != "b" || all[2].ID != "c" {
		t.Errorf("not sorted: %v", all)
	}
}
