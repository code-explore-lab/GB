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

func TestTouchRefreshesLastSeen(t *testing.T) {
	r := New()
	id := "dev"
	old := time.Now().Add(-time.Minute)
	r.Upsert(Device{ID: id, LastSeen: old, Online: false})

	if ok := r.Touch(id, &net.UDPAddr{Port: 5070}, time.Now()); !ok {
		t.Fatal("Touch returned false for registered device")
	}

	d, _ := r.Get(id)
	if !d.Online {
		t.Error("device should be online after keepalive")
	}
	if d.LastSeen.Equal(old) {
		t.Error("LastSeen was not refreshed")
	}
	if d.Addr.Port != 5070 {
		t.Errorf("Addr.Port = %d, want 5070", d.Addr.Port)
	}
}

func TestTouchUnknownDevice(t *testing.T) {
	r := New()
	if ok := r.Touch("nobody", nil, time.Now()); ok {
		t.Fatal("Touch should return false for unknown device")
	}
}

func TestMarkStaleOffline(t *testing.T) {
	r := New()
	now := time.Now()
	timeout := 90 * time.Second

	r.Upsert(Device{ID: "alive", LastSeen: now.Add(-10 * time.Second), Online: true})
	r.Upsert(Device{ID: "stale", LastSeen: now.Add(-2 * time.Minute), Online: true})
	r.Upsert(Device{ID: "already-offline", LastSeen: now.Add(-2 * time.Minute), Online: false})

	if got := r.MarkStaleOffline(now, timeout); len(got) != 1 || got[0] != "stale" {
		t.Fatalf("MarkStaleOffline = %v, want [stale]", got)
	}

	// 第二次调用不应该重复返回（幂等）。
	if got := r.MarkStaleOffline(now, timeout); len(got) != 0 {
		t.Fatalf("second call = %v, want empty", got)
	}

	alive, _ := r.Get("alive")
	if !alive.Online {
		t.Error("alive device must stay online")
	}
	stale, _ := r.Get("stale")
	if stale.Online {
		t.Error("stale device must be offline")
	}
}

func TestTouchBringsDeviceBackOnline(t *testing.T) {
	r := New()
	id := "dev"
	r.Upsert(Device{ID: id, LastSeen: time.Now().Add(-time.Hour), Online: false})

	if ok := r.Touch(id, nil, time.Now()); !ok {
		t.Fatal("Touch failed")
	}
	d, _ := r.Get(id)
	if !d.Online {
		t.Error("device should be back online after keepalive")
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
