// Package registry 保存已注册设备的内存状态。
//
// Task 5 只需要 ID / 地址 / 过期时间；
// Task 6 会用 LastSeen 判断设备是否离线。
package registry

import (
	"net"
	"sort"
	"sync"
	"time"
)

// Device 是一个已注册设备。
type Device struct {
	ID        string
	Addr      *net.UDPAddr
	ExpiresAt time.Time // 注册有效期截止时间
	LastSeen  time.Time // 最后一次收到它的消息（保活用）
	Online    bool
}

// Registry 是并发安全的设备表。
type Registry struct {
	mu      sync.RWMutex
	devices map[string]Device
}

func New() *Registry {
	return &Registry{devices: make(map[string]Device)}
}

// Upsert 新增或更新设备（设备重注册时走这里）。
func (r *Registry) Upsert(d Device) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.devices[d.ID] = d
}

// Get 按设备 ID 查询。
func (r *Registry) Get(id string) (Device, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.devices[id]
	return d, ok
}

// Touch 刷新设备的保活时间，并把它标记为在线。
// 返回 false 表示这个设备没注册过（保活消息来自未知设备）。
func (r *Registry) Touch(id string, addr *net.UDPAddr, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	d, ok := r.devices[id]
	if !ok {
		return false
	}
	d.LastSeen = now
	d.Online = true
	if addr != nil {
		d.Addr = addr // 设备可能换了端口（NAT 重新映射），跟着更新
	}
	r.devices[id] = d
	return true
}

// MarkStaleOffline 把所有"在线但超过 timeout 没有保活"的设备标记为离线，
// 返回本次状态发生变化的设备 ID（按字母序，方便日志稳定）。
//
// 这个函数是幂等的：已经是离线的设备不会被重复返回。
func (r *Registry) MarkStaleOffline(now time.Time, timeout time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var offline []string
	for id, d := range r.devices {
		if d.Online && now.Sub(d.LastSeen) > timeout {
			d.Online = false
			r.devices[id] = d
			offline = append(offline, id)
		}
	}
	sort.Strings(offline)
	return offline
}

// All 返回按 ID 排序的快照。
// 返回副本，调用方遍历时不会阻塞写入。
func (r *Registry) All() []Device {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Device, 0, len(r.devices))
	for _, d := range r.devices {
		list = append(list, d)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// Len 返回设备数量。
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.devices)
}
