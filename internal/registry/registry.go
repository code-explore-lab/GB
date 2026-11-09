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
