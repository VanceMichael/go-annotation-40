package constellation_test

import (
	"errors"
	"testing"

	"satnet/internal/constellation"
	"satnet/internal/model"
	"satnet/internal/seed"
)

func newRegistry(t *testing.T) *constellation.Registry {
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

func TestLookup(t *testing.T) {
	reg := newRegistry(t)
	if _, err := reg.Satellite("SAT-2401"); err != nil {
		t.Fatalf("查询卫星失败: %v", err)
	}
	if _, err := reg.Satellite("SAT-NOPE"); !errors.Is(err, model.ErrSatelliteNotFound) {
		t.Fatalf("应返回 ErrSatelliteNotFound, 得到 %v", err)
	}
	if _, err := reg.Station("GS-KS"); err != nil {
		t.Fatalf("查询地面站失败: %v", err)
	}
	if _, err := reg.Station("GS-NOPE"); !errors.Is(err, model.ErrStationNotFound) {
		t.Fatalf("应返回 ErrStationNotFound, 得到 %v", err)
	}
}

func TestTrackableIncludesSafeMode(t *testing.T) {
	reg := newRegistry(t)
	ids := reg.TrackableSatellites()
	if len(ids) != 3 {
		t.Fatalf("可测控卫星应为 3 颗, 实际 %v", ids)
	}
	if err := reg.CheckTrackable("SAT-2403"); err != nil {
		t.Fatalf("安全模式卫星应可测控: %v", err)
	}
	if err := reg.CheckTrackable("SAT-2404"); !errors.Is(err, model.ErrSatelliteNotOperational) {
		t.Fatalf("在轨测试卫星不应可测控, 得到 %v", err)
	}
	if err := reg.CheckTrackable("SAT-2405"); !errors.Is(err, model.ErrSatelliteNotOperational) {
		t.Fatalf("已离轨卫星不应可测控, 得到 %v", err)
	}
}

func TestCheckBand(t *testing.T) {
	reg := newRegistry(t)
	if err := reg.CheckBand("SAT-2402", "GS-SY", model.BandKu); err != nil {
		t.Fatalf("双方都支持 Ku 时应通过: %v", err)
	}
	if err := reg.CheckBand("SAT-2403", "GS-SY", model.BandKa); !errors.Is(err, model.ErrInvalidGrant) {
		t.Fatalf("卫星不支持该频段应返回 ErrInvalidGrant, 得到 %v", err)
	}
	if err := reg.CheckBand("SAT-2402", "GS-MOB", model.BandKu); !errors.Is(err, model.ErrInvalidGrant) {
		t.Fatalf("地面站不支持该频段应返回 ErrInvalidGrant, 得到 %v", err)
	}
}

func TestCountByStateCoversAll(t *testing.T) {
	reg := newRegistry(t)
	counts := reg.CountByState()
	total := 0
	for _, s := range model.AllSatStates() {
		if _, ok := counts[s]; !ok {
			t.Fatalf("统计应覆盖状态 %s", s)
		}
		total += counts[s]
	}
	if total != len(seed.Satellites()) {
		t.Fatalf("分状态合计 %d 与卫星数 %d 不一致", total, len(seed.Satellites()))
	}
}

func TestAntennasSum(t *testing.T) {
	reg := newRegistry(t)
	want := 0
	for _, s := range seed.Stations() {
		want += s.Antennas
	}
	if got := reg.Antennas(); got != want {
		t.Fatalf("天线总数应为 %d, 实际 %d", want, got)
	}
}

func TestAddRejectsInvalid(t *testing.T) {
	reg := constellation.NewRegistry()
	if err := reg.AddSatellite(model.Satellite{ID: ""}); !errors.Is(err, model.ErrInvalidSatellite) {
		t.Fatalf("空编号应返回 ErrInvalidSatellite, 得到 %v", err)
	}
	if err := reg.AddStation(model.Station{ID: "GS", Kind: model.StationTTC,
		LatDeg: 0, LonDeg: 0, Antennas: 0, Bands: []model.Band{model.BandS}}); !errors.Is(err, model.ErrInvalidStation) {
		t.Fatalf("天线数为 0 应返回 ErrInvalidStation, 得到 %v", err)
	}
}

func TestDescribeNotEmpty(t *testing.T) {
	reg := newRegistry(t)
	sat, _ := reg.Satellite("SAT-2401")
	st, _ := reg.Station("GS-SY")
	if constellation.DescribeSatellite(sat) == "" || constellation.DescribeStation(st) == "" {
		t.Fatal("描述不应为空")
	}
}
