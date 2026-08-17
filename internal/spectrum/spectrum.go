// Package spectrum 负责频轨资源（频段带宽）的并发分配与释放。
package spectrum

import (
	"fmt"
	"sort"
	"sync"

	"satnet/internal/model"
)

// Grant 是一次频轨授权。
type Grant struct {
	ID          string     `json:"id"`
	SatelliteID string     `json:"satellite_id"`
	Band        model.Band `json:"band"`
	// MHz 是本次授权占用的带宽，单位兆赫。
	MHz int `json:"mhz"`
}

func bandIndex(b model.Band) int {
	for i, v := range model.AllBands() {
		if v == b {
			return i
		}
	}
	return -1
}

// Allocator 是频轨资源分配器。
//
// 各频段总带宽由 model.Band.TotalMHz 给出，任何时刻已授权带宽都不得超过总带宽。
type Allocator struct {
	mu      sync.RWMutex
	usedMHz []int
	grants  []Grant
	seq     int
	rejects int
}

// NewAllocator 构造频轨资源分配器。
func NewAllocator() *Allocator {
	return &Allocator{usedMHz: make([]int, len(model.AllBands()))}
}

// Allocate 为某颗卫星在指定频段上申请带宽。
//
// 剩余带宽不足时返回 model.ErrBandExhausted 并计入拒绝数，不改动已授权量。
func (a *Allocator) Allocate(satelliteID string, band model.Band, mhz int) (Grant, error) {
	i := bandIndex(band)
	if i < 0 {
		return Grant{}, fmt.Errorf("%w: %q", model.ErrUnknownBand, band)
	}
	if mhz <= 0 {
		return Grant{}, fmt.Errorf("%w: 申请带宽必须为正", model.ErrInvalidGrant)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.usedMHz[i]+mhz > band.TotalMHz() {
		a.rejects++
		return Grant{}, fmt.Errorf("%w: %s 需要 %dMHz, 剩余 %dMHz",
			model.ErrBandExhausted, band.DisplayName(), mhz, band.TotalMHz()-a.usedMHz[i])
	}

	a.seq++
	g := Grant{
		ID:          fmt.Sprintf("G-%06d", a.seq),
		SatelliteID: satelliteID,
		Band:        band,
		MHz:         mhz,
	}
	a.usedMHz[i] += mhz
	a.grants = append(a.grants, g)
	return g, nil
}

// Release 释放一次授权占用的带宽。
func (a *Allocator) Release(grantID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for k, g := range a.grants {
		if g.ID != grantID {
			continue
		}
		i := bandIndex(g.Band)
		if i >= 0 {
			a.usedMHz[i] -= g.MHz
		}
		a.grants = append(a.grants[:k:k], a.grants[k+1:]...)
		return nil
	}
	return fmt.Errorf("%w: 授权 %s 不存在", model.ErrInvalidGrant, grantID)
}

// UsedMHz 返回某频段已授权带宽。
func (a *Allocator) UsedMHz(band model.Band) int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	i := bandIndex(band)
	if i < 0 {
		return 0
	}
	return a.usedMHz[i]
}

// Grants 返回全部授权的独立副本，按授权编号排序。
func (a *Allocator) Grants() []Grant {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]Grant, len(a.grants))
	copy(out, a.grants)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Rejects 返回被拒绝的申请次数。
func (a *Allocator) Rejects() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rejects
}

// GrantedMHz 返回全部授权记录累加出的带宽。
func (a *Allocator) GrantedMHz(band model.Band) int {
	total := 0
	for _, g := range a.Grants() {
		if g.Band == band {
			total += g.MHz
		}
	}
	return total
}

// Verify 交叉核对已授权带宽与授权记录。
//
// 每个频段的已授权带宽必须等于该频段全部授权记录之和，且不得超过总带宽。
func (a *Allocator) Verify() error {
	for _, b := range model.AllBands() {
		used := a.UsedMHz(b)
		granted := a.GrantedMHz(b)
		if used != granted {
			return fmt.Errorf("%w: %s 已授权 %dMHz, 授权记录合计 %dMHz",
				model.ErrBandOversubscribed, b.DisplayName(), used, granted)
		}
		if used > b.TotalMHz() {
			return fmt.Errorf("%w: %s 已授权 %dMHz, 总带宽 %dMHz",
				model.ErrBandOversubscribed, b.DisplayName(), used, b.TotalMHz())
		}
	}
	return nil
}

// Usage 是某频段的占用情况。
type Usage struct {
	Band     string `json:"band"`
	TotalMHz int    `json:"total_mhz"`
	UsedMHz  int    `json:"used_mhz"`
	// GrantedMHz 是授权记录累加出的带宽，正常情况下等于 UsedMHz。
	GrantedMHz int `json:"granted_mhz"`
	Grants     int `json:"grants"`
}

// Snapshot 返回各频段占用情况。
func (a *Allocator) Snapshot() []Usage {
	grants := a.Grants()
	counts := make(map[model.Band]int)
	sums := make(map[model.Band]int)
	for _, g := range grants {
		counts[g.Band]++
		sums[g.Band] += g.MHz
	}
	out := make([]Usage, 0, len(model.AllBands()))
	for _, b := range model.AllBands() {
		out = append(out, Usage{
			Band:       string(b),
			TotalMHz:   b.TotalMHz(),
			UsedMHz:    a.UsedMHz(b),
			GrantedMHz: sums[b],
			Grants:     counts[b],
		})
	}
	return out
}

// Describe 返回授权的单行描述。
func Describe(g Grant) string {
	return fmt.Sprintf("%s %s %s %dMHz", g.ID, g.SatelliteID, g.Band.DisplayName(), g.MHz)
}
