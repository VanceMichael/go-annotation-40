package pass_test

import (
	"testing"
	"time"

	"satnet/internal/model"
	"satnet/internal/pass"
	"satnet/internal/seed"
)

func passByID(t *testing.T, id string) model.Pass {
	t.Helper()
	for _, p := range seed.Passes() {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("样例数据缺少可见弧 %s", id)
	return model.Pass{}
}

func TestSampleHasCrossBoundaryPasses(t *testing.T) {
	got := 0
	for _, p := range seed.Passes() {
		if pass.CrossesDayBoundary(p) {
			got++
		}
	}
	if got != len(seed.CrossBoundaryPassIDs()) {
		t.Fatalf("跨日界可见弧应为 %d 段, 实际 %d 段",
			len(seed.CrossBoundaryPassIDs()), got)
	}
}

func TestClipToDaySplitsCrossBoundaryPass(t *testing.T) {
	// PASS-004: 2026-08-16 23:50 -> 2026-08-17 00:10，两日各 10 分钟。
	p := passByID(t, "PASS-004")

	first, ok := pass.ClipToDay(p, "2026-08-16")
	if !ok {
		t.Fatal("跨日界可见弧在起始日应当有片段")
	}
	if got := first.Duration(); got != 10*time.Minute {
		t.Fatalf("起始日片段应为 10 分钟, 实际 %v", got)
	}

	second, ok := pass.ClipToDay(p, "2026-08-17")
	if !ok {
		t.Fatalf("跨日界可见弧在次日应当有片段，实际被整段丢弃")
	}
	if got := second.Duration(); got != 10*time.Minute {
		t.Fatalf("次日片段应为 10 分钟, 实际 %v", got)
	}
	if first.Duration()+second.Duration() != p.Duration() {
		t.Fatalf("两日片段合计 %v 应等于原时长 %v",
			first.Duration()+second.Duration(), p.Duration())
	}
}

func TestClipToDayKeepsWhollyContainedPass(t *testing.T) {
	p := passByID(t, "PASS-001")
	got, ok := pass.ClipToDay(p, "2026-08-16")
	if !ok {
		t.Fatal("当日内的可见弧应当保留")
	}
	if !got.Start.Equal(p.Start) || !got.End.Equal(p.End) {
		t.Fatalf("当日内的可见弧不应被裁剪: %v -> %v", got.Start, got.End)
	}
}

func TestClipToDayDropsUnrelatedDay(t *testing.T) {
	p := passByID(t, "PASS-001")
	if _, ok := pass.ClipToDay(p, "2026-08-20"); ok {
		t.Fatal("与该自然日不相交时应返回 false")
	}
}

func TestOnDayIncludesTailOfPreviousDay(t *testing.T) {
	// PASS-007: 2026-08-15 23:55 -> 2026-08-16 00:05，基准日应当看到 5 分钟片段。
	got := pass.OnDay(seed.Passes(), "2026-08-16")
	found := false
	for _, p := range got {
		if p.ID != "PASS-007" {
			continue
		}
		found = true
		if p.Duration() != 5*time.Minute {
			t.Fatalf("前一日延续过来的片段应为 5 分钟, 实际 %v", p.Duration())
		}
	}
	if !found {
		t.Fatal("基准日应当包含由前一日延续过来的可见弧片段")
	}
}

func TestCoverageByDayConservesTotal(t *testing.T) {
	passes := seed.Passes()
	byDay := pass.CoverageByDay(passes, seed.Days())
	var sum int64
	for _, d := range byDay {
		sum += d.SecondsTotal
	}
	if want := pass.TotalSeconds(passes); sum != want {
		t.Fatalf("按自然日裁剪后累计 %d 秒, 可见弧累计 %d 秒；裁剪不应丢失或重复时长", sum, want)
	}
}

func TestCoverageOnDayMatchesClips(t *testing.T) {
	passes := seed.Passes()
	for _, day := range seed.Days() {
		var want time.Duration
		for _, p := range passes {
			if clipped, ok := pass.ClipToDay(p, day); ok {
				want += clipped.Duration()
			}
		}
		if got := pass.CoverageOnDay(passes, day); got != want {
			t.Fatalf("%s 覆盖时长应为 %v, 实际 %v", day, want, got)
		}
	}
}

func TestSpanningDaysCoversBothSides(t *testing.T) {
	p := passByID(t, "PASS-004")
	days := pass.SpanningDays(p)
	if len(days) != 2 {
		t.Fatalf("跨日界可见弧应跨越 2 个自然日, 实际 %v", days)
	}
	if days[0] != "2026-08-16" || days[1] != "2026-08-17" {
		t.Fatalf("跨越的自然日不符: %v", days)
	}
}

func TestConflictsIgnoresTouchingEndpoints(t *testing.T) {
	base := time.Date(2026, 8, 16, 1, 0, 0, 0, time.UTC)
	items := []model.Pass{
		{ID: "A", SatelliteID: "S1", StationID: "G1", Start: base, End: base.Add(10 * time.Minute), MaxElevationDeg: 30},
		{ID: "B", SatelliteID: "S2", StationID: "G1", Start: base.Add(10 * time.Minute), End: base.Add(20 * time.Minute), MaxElevationDeg: 30},
	}
	if got := pass.Conflicts(items); got != 0 {
		t.Fatalf("端点相接不应算冲突, 实际 %d", got)
	}
	items[1].Start = base.Add(5 * time.Minute)
	if got := pass.Conflicts(items); got != 1 {
		t.Fatalf("重叠应算 1 对冲突, 实际 %d", got)
	}
}

func TestParseDayRejectsBadInput(t *testing.T) {
	if _, err := pass.ParseDay("2026/08/16"); err == nil {
		t.Fatal("非法自然日格式应当报错")
	}
}
