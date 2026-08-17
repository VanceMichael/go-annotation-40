// Package report 汇总星座规模、可见弧覆盖与频轨占用报表。
package report

import (
	"fmt"

	"satnet/internal/constellation"
	"satnet/internal/model"
	"satnet/internal/pass"
	"satnet/internal/spectrum"
	"satnet/internal/window"
)

// ScaleSummary 是星座规模汇总。
type ScaleSummary struct {
	Satellites int `json:"satellites"`
	// Trackable 是可安排测控窗口的卫星数。
	Trackable int            `json:"trackable"`
	ByState   map[string]int `json:"by_state"`
	Stations  int            `json:"stations"`
	Antennas  int            `json:"antennas"`
}

// Scale 生成星座规模报表。
func Scale(reg *constellation.Registry) ScaleSummary {
	counts := reg.CountByState()
	byState := make(map[string]int, len(counts))
	for _, s := range model.AllSatStates() {
		byState[string(s)] = counts[s]
	}
	return ScaleSummary{
		Satellites: len(reg.Satellites()),
		Trackable:  len(reg.TrackableSatellites()),
		ByState:    byState,
		Stations:   len(reg.Stations()),
		Antennas:   reg.Antennas(),
	}
}

// CoverageSummary 是可见弧覆盖汇总。
type CoverageSummary struct {
	Passes int `json:"passes"`
	// CrossBoundary 是跨越 UTC 日界的可见弧数。
	CrossBoundary int `json:"cross_boundary"`
	// SecondsTotal 是全部可见弧的累计秒数。
	SecondsTotal int64 `json:"seconds_total"`
	// SecondsByDay 是按自然日裁剪后各日累计秒数之和。
	SecondsByDay int64 `json:"seconds_by_day"`
	// Balanced 报告按日裁剪后的累计秒数是否等于全部可见弧的累计秒数。
	Balanced  bool               `json:"balanced"`
	Days      []pass.DayCoverage `json:"days"`
	Conflicts int                `json:"conflicts"`
}

// Coverage 生成可见弧覆盖报表。
//
// 按自然日裁剪只是把跨日界的可见弧分摊到相邻两日，
// 因此各日累计秒数之和必须等于全部可见弧的累计秒数。
func Coverage(passes []model.Pass, days []string) CoverageSummary {
	cross := 0
	for _, p := range passes {
		if pass.CrossesDayBoundary(p) {
			cross++
		}
	}
	byDay := pass.CoverageByDay(passes, days)
	var sum int64
	for _, d := range byDay {
		sum += d.SecondsTotal
	}
	total := pass.TotalSeconds(passes)
	return CoverageSummary{
		Passes:        len(passes),
		CrossBoundary: cross,
		SecondsTotal:  total,
		SecondsByDay:  sum,
		Balanced:      sum == total,
		Days:          byDay,
		Conflicts:     pass.Conflicts(passes),
	}
}

// ScheduleSummary 是测控时隙分组汇总。
type ScheduleSummary struct {
	Slots int `json:"slots"`
	// Groups 是分组数，即涉及的卫星数。
	Groups int `json:"groups"`
	// Misassigned 是归属错误的时隙数。
	Misassigned int `json:"misassigned"`
	// DistinctSlots 是互异时隙编号数。
	DistinctSlots int `json:"distinct_slots"`
	// Consistent 报告分组是否自洽。
	Consistent bool `json:"consistent"`
}

// Schedule 生成测控时隙分组报表。
func Schedule(groups map[string][]window.Slot) ScheduleSummary {
	return ScheduleSummary{
		Slots:         window.TotalSlots(groups),
		Groups:        len(groups),
		Misassigned:   window.Misassigned(groups),
		DistinctSlots: window.DistinctSlotIDs(groups),
		Consistent:    window.Verify(groups) == nil,
	}
}

// SpectrumSummary 是频轨占用汇总。
type SpectrumSummary struct {
	Usage []spectrum.Usage `json:"usage"`
	// Grants 是授权记录条数。
	Grants int `json:"grants"`
	// Rejects 是被拒绝的申请次数。
	Rejects int `json:"rejects"`
	// Consistent 报告已授权带宽与授权记录是否一致。
	Consistent bool `json:"consistent"`
}

// Spectrum 生成频轨占用报表。
func Spectrum(a *spectrum.Allocator) SpectrumSummary {
	return SpectrumSummary{
		Usage:      a.Snapshot(),
		Grants:     len(a.Grants()),
		Rejects:    a.Rejects(),
		Consistent: a.Verify() == nil,
	}
}

// Describe 返回星座规模汇总的单行描述。
func Describe(s ScaleSummary) string {
	return fmt.Sprintf("卫星%d（可测控%d） 地面站%d 天线%d",
		s.Satellites, s.Trackable, s.Stations, s.Antennas)
}
