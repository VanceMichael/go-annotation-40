package report_test

import (
	"testing"
	"time"

	"satnet/internal/constellation"
	"satnet/internal/model"
	"satnet/internal/pass"
	"satnet/internal/report"
	"satnet/internal/seed"
	"satnet/internal/spectrum"
	"satnet/internal/window"
)

func registry(t *testing.T) *constellation.Registry {
	t.Helper()
	reg := constellation.NewRegistry()
	if err := reg.AddSatellites(seed.Satellites()); err != nil {
		t.Fatalf("装载卫星失败: %v", err)
	}
	if err := reg.AddStations(seed.Stations()); err != nil {
		t.Fatalf("装载地面站失败: %v", err)
	}
	return reg
}

func TestScaleSummary(t *testing.T) {
	s := report.Scale(registry(t))
	if s.Satellites != len(seed.Satellites()) {
		t.Fatalf("卫星数应为 %d, 实际 %d", len(seed.Satellites()), s.Satellites)
	}
	if s.Stations != len(seed.Stations()) {
		t.Fatalf("地面站数应为 %d, 实际 %d", len(seed.Stations()), s.Stations)
	}
	if s.Trackable != 3 {
		t.Fatalf("可测控卫星应为 3 颗, 实际 %d", s.Trackable)
	}
	total := 0
	for _, st := range model.AllSatStates() {
		total += s.ByState[string(st)]
	}
	if total != s.Satellites {
		t.Fatalf("分状态合计 %d 与卫星数 %d 不一致", total, s.Satellites)
	}
	if report.Describe(s) == "" {
		t.Fatal("描述不应为空")
	}
}

func TestCoverageBalanced(t *testing.T) {
	sum := report.Coverage(seed.Passes(), seed.Days())
	if !sum.Balanced {
		t.Fatalf("按自然日裁剪后应守恒: 按日 %d 秒 / 合计 %d 秒",
			sum.SecondsByDay, sum.SecondsTotal)
	}
	if sum.CrossBoundary != len(seed.CrossBoundaryPassIDs()) {
		t.Fatalf("跨日界可见弧应为 %d 段, 实际 %d 段",
			len(seed.CrossBoundaryPassIDs()), sum.CrossBoundary)
	}
	if len(sum.Days) != len(seed.Days()) {
		t.Fatalf("应覆盖 %d 个自然日, 实际 %d", len(seed.Days()), len(sum.Days))
	}
}

func TestCoverageDaysMatchPassPackage(t *testing.T) {
	passes := seed.Passes()
	sum := report.Coverage(passes, seed.Days())
	for _, d := range sum.Days {
		want := int64(pass.CoverageOnDay(passes, d.Day) / time.Second)
		if d.SecondsTotal != want {
			t.Fatalf("%s 覆盖应为 %d 秒, 实际 %d 秒", d.Day, want, d.SecondsTotal)
		}
	}
}

func TestScheduleSummaryConsistent(t *testing.T) {
	groups := window.Groups(window.SplitAll(seed.Passes(), 5*time.Minute, 120))
	sum := report.Schedule(groups)
	if !sum.Consistent {
		t.Fatalf("分组应自洽: %+v", sum)
	}
	if sum.Misassigned != 0 {
		t.Fatalf("不应有归属错误的时隙: %d", sum.Misassigned)
	}
	if sum.DistinctSlots != sum.Slots {
		t.Fatalf("互异时隙 %d 应等于时隙总数 %d", sum.DistinctSlots, sum.Slots)
	}
}

func TestSpectrumSummaryConsistent(t *testing.T) {
	a := spectrum.NewAllocator()
	for i, b := range seed.BandRequests(9) {
		if _, err := a.Allocate(seed.SatelliteIDFor(i), b, 20); err != nil {
			t.Fatalf("分配失败: %v", err)
		}
	}
	sum := report.Spectrum(a)
	if !sum.Consistent {
		t.Fatalf("频轨占用应自洽: %+v", sum)
	}
	if sum.Grants != 9 {
		t.Fatalf("授权记录应为 9 条, 实际 %d", sum.Grants)
	}
	for _, u := range sum.Usage {
		if u.UsedMHz != u.GrantedMHz {
			t.Fatalf("频段 %s 已授权 %d 与记录 %d 不一致", u.Band, u.UsedMHz, u.GrantedMHz)
		}
	}
}
