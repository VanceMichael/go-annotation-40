package model_test

import (
	"errors"
	"testing"
	"time"

	"satnet/internal/model"
)

func TestParseBandRoundTrip(t *testing.T) {
	for _, b := range model.AllBands() {
		got, err := model.ParseBand(string(b))
		if err != nil || got != b || got.DisplayName() == "" {
			t.Fatalf("频段 %s 解析异常: %s %v", b, got, err)
		}
		if b.TotalMHz() <= 0 {
			t.Fatalf("频段 %s 总带宽应为正", b)
		}
	}
	if _, err := model.ParseBand("x"); !errors.Is(err, model.ErrUnknownBand) {
		t.Fatalf("未知频段应返回 ErrUnknownBand, 得到 %v", err)
	}
}

func TestParseSatStateRoundTrip(t *testing.T) {
	for _, s := range model.AllSatStates() {
		got, err := model.ParseSatState(string(s))
		if err != nil || got != s || got.DisplayName() == "" {
			t.Fatalf("状态 %s 解析异常: %s %v", s, got, err)
		}
	}
	if _, err := model.ParseSatState("orbiting"); !errors.Is(err, model.ErrUnknownSatState) {
		t.Fatalf("未知状态应返回 ErrUnknownSatState, 得到 %v", err)
	}
}

func TestTrackableStates(t *testing.T) {
	if !model.SatOperational.Trackable() || !model.SatSafeMode.Trackable() {
		t.Fatal("在轨运行与安全模式都应可测控")
	}
	for _, s := range []model.SatState{model.SatLaunched, model.SatCommissioning, model.SatDeorbited} {
		if s.Trackable() {
			t.Fatalf("状态 %s 不应可测控", s)
		}
	}
}

func TestParseStationKindRoundTrip(t *testing.T) {
	for _, k := range model.AllStationKinds() {
		got, err := model.ParseStationKind(string(k))
		if err != nil || got != k || got.DisplayName() == "" {
			t.Fatalf("类型 %s 解析异常: %s %v", k, got, err)
		}
	}
	if _, err := model.ParseStationKind("ship"); !errors.Is(err, model.ErrUnknownStationKind) {
		t.Fatalf("未知类型应返回 ErrUnknownStationKind, 得到 %v", err)
	}
}

func satFixture() model.Satellite {
	return model.Satellite{
		ID: "SAT-1", Name: "星", Plane: "P-1", State: model.SatOperational,
		AltitudeKM: 1000, InclinationDeg: 86, Bands: []model.Band{model.BandS},
	}
}

func TestSatelliteValidate(t *testing.T) {
	if err := satFixture().Validate(); err != nil {
		t.Fatalf("合法卫星应通过: %v", err)
	}
	for _, mut := range []func(*model.Satellite){
		func(s *model.Satellite) { s.AltitudeKM = 0 },
		func(s *model.Satellite) { s.InclinationDeg = 200 },
		func(s *model.Satellite) { s.Bands = nil },
		func(s *model.Satellite) { s.Plane = "" },
	} {
		bad := satFixture()
		mut(&bad)
		if err := bad.Validate(); !errors.Is(err, model.ErrInvalidSatellite) {
			t.Fatalf("非法卫星应返回 ErrInvalidSatellite, 得到 %v", err)
		}
	}
}

func TestSatelliteSupports(t *testing.T) {
	s := satFixture()
	if !s.Supports(model.BandS) {
		t.Fatal("应支持已登记频段")
	}
	if s.Supports(model.BandKa) {
		t.Fatal("不应支持未登记频段")
	}
}

func passFixture() model.Pass {
	base := time.Date(2026, 8, 16, 1, 0, 0, 0, time.UTC)
	return model.Pass{
		ID: "P-1", SatelliteID: "SAT-1", StationID: "GS-1",
		Start: base, End: base.Add(10 * time.Minute), MaxElevationDeg: 40,
	}
}

func TestPassValidateAndDuration(t *testing.T) {
	p := passFixture()
	if err := p.Validate(); err != nil {
		t.Fatalf("合法可见弧应通过: %v", err)
	}
	if p.Duration() != 10*time.Minute {
		t.Fatalf("时长应为 10 分钟, 实际 %v", p.Duration())
	}
	bad := passFixture()
	bad.End = bad.Start
	if err := bad.Validate(); !errors.Is(err, model.ErrInvalidPass) {
		t.Fatalf("结束不晚于开始应返回 ErrInvalidPass, 得到 %v", err)
	}
	bad = passFixture()
	bad.MaxElevationDeg = 100
	if err := bad.Validate(); !errors.Is(err, model.ErrInvalidPass) {
		t.Fatalf("仰角越界应返回 ErrInvalidPass, 得到 %v", err)
	}
}

func TestPassOverlapsExcludesTouchingEndpoints(t *testing.T) {
	a := passFixture()
	b := passFixture()
	b.ID = "P-2"
	b.Start = a.End
	b.End = a.End.Add(5 * time.Minute)
	if a.Overlaps(b) || b.Overlaps(a) {
		t.Fatal("端点相接不应算重叠")
	}
	b.Start = a.End.Add(-time.Minute)
	if !a.Overlaps(b) {
		t.Fatal("时间相交应算重叠")
	}
}

func TestFrameValidateAndChecksum(t *testing.T) {
	f := model.Frame{ID: "F-1", SatelliteID: "SAT-1", PassID: "P-1",
		At: time.Date(2026, 8, 16, 1, 0, 0, 0, time.UTC), Bytes: 128}
	f.Checksum = f.ExpectedChecksum()
	if err := f.Validate(); err != nil {
		t.Fatalf("合法帧应通过: %v", err)
	}
	if f.Checksum != f.ExpectedChecksum() {
		t.Fatal("校验和应可复算")
	}
	bad := f
	bad.Bytes = 0
	if err := bad.Validate(); !errors.Is(err, model.ErrInvalidFrame) {
		t.Fatalf("长度为 0 应返回 ErrInvalidFrame, 得到 %v", err)
	}
}

func TestUTCDay(t *testing.T) {
	tm := time.Date(2026, 8, 16, 23, 59, 0, 0, time.UTC)
	if got := model.UTCDay(tm); got != "2026-08-16" {
		t.Fatalf("自然日应为 2026-08-16, 实际 %s", got)
	}
}

func TestSortPassesStable(t *testing.T) {
	base := time.Date(2026, 8, 16, 1, 0, 0, 0, time.UTC)
	items := []model.Pass{
		{ID: "B", SatelliteID: "S2", Start: base.Add(time.Hour), End: base.Add(2 * time.Hour)},
		{ID: "A", SatelliteID: "S1", Start: base, End: base.Add(time.Minute)},
		{ID: "C", SatelliteID: "S1", Start: base, End: base.Add(time.Minute)},
	}
	model.SortPasses(items)
	if items[0].ID != "A" || items[1].ID != "C" || items[2].ID != "B" {
		t.Fatalf("排序结果不符: %v", items)
	}
}
