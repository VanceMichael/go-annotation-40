// Package seed 提供内置样例数据，仅用于本地演练。
package seed

import (
	"fmt"
	"time"

	"satnet/internal/model"
)

func ts(day, hour, minute int) time.Time {
	return time.Date(2026, 8, day, hour, minute, 0, 0, time.UTC)
}

// Now 返回样例数据使用的固定当前时间。
func Now() time.Time {
	return ts(16, 9, 0)
}

// Day 返回样例数据的基准 UTC 自然日。
func Day() string {
	return "2026-08-16"
}

// NextDay 返回基准自然日的次日。
func NextDay() string {
	return "2026-08-17"
}

// Days 返回样例数据覆盖的自然日。
func Days() []string {
	return []string{"2026-08-15", "2026-08-16", "2026-08-17"}
}

// Satellites 返回样例卫星。
func Satellites() []model.Satellite {
	return []model.Satellite{
		{ID: "SAT-2401", Name: "低轨互联网星-2401", Plane: "P-01", State: model.SatOperational,
			AltitudeKM: 1080, InclinationDeg: 86.5,
			Bands: []model.Band{model.BandS, model.BandKa}},
		{ID: "SAT-2402", Name: "低轨互联网星-2402", Plane: "P-01", State: model.SatOperational,
			AltitudeKM: 1080, InclinationDeg: 86.5,
			Bands: []model.Band{model.BandS, model.BandKu, model.BandKa}},
		{ID: "SAT-2403", Name: "低轨互联网星-2403", Plane: "P-02", State: model.SatSafeMode,
			AltitudeKM: 1075, InclinationDeg: 86.4,
			Bands: []model.Band{model.BandS}},
		{ID: "SAT-2404", Name: "低轨互联网星-2404", Plane: "P-02", State: model.SatCommissioning,
			AltitudeKM: 1082, InclinationDeg: 86.5,
			Bands: []model.Band{model.BandS, model.BandKa, model.BandQV}},
		{ID: "SAT-2405", Name: "低轨互联网星-2405", Plane: "P-03", State: model.SatDeorbited,
			AltitudeKM: 990, InclinationDeg: 86.2,
			Bands: []model.Band{model.BandS}},
	}
}

// Stations 返回样例地面站。
func Stations() []model.Station {
	return []model.Station{
		{ID: "GS-KS", Name: "喀什测控站", Kind: model.StationTTC,
			LatDeg: 39.47, LonDeg: 75.98, Antennas: 4,
			Bands: []model.Band{model.BandS, model.BandKa}},
		{ID: "GS-SY", Name: "三亚信关站", Kind: model.StationGateway,
			LatDeg: 18.25, LonDeg: 109.51, Antennas: 6,
			Bands: []model.Band{model.BandS, model.BandKu, model.BandKa, model.BandQV}},
		{ID: "GS-MH", Name: "漠河测控站", Kind: model.StationTTC,
			LatDeg: 53.47, LonDeg: 122.34, Antennas: 3,
			Bands: []model.Band{model.BandS, model.BandKa}},
		{ID: "GS-MOB", Name: "车载机动站-01", Kind: model.StationMobile,
			LatDeg: 43.82, LonDeg: 87.62, Antennas: 1,
			Bands: []model.Band{model.BandS}},
	}
}

// Passes 返回样例可见弧。
//
// PASS-004 与 PASS-007 故意跨越 UTC 日界，用于演练按自然日归集时的裁剪。
func Passes() []model.Pass {
	return []model.Pass{
		{ID: "PASS-001", SatelliteID: "SAT-2401", StationID: "GS-KS",
			Start: ts(16, 2, 10), End: ts(16, 2, 22), MaxElevationDeg: 62.4},
		{ID: "PASS-002", SatelliteID: "SAT-2401", StationID: "GS-MH",
			Start: ts(16, 3, 40), End: ts(16, 3, 51), MaxElevationDeg: 48.1},
		{ID: "PASS-003", SatelliteID: "SAT-2402", StationID: "GS-SY",
			Start: ts(16, 5, 5), End: ts(16, 5, 19), MaxElevationDeg: 71.6},
		// 跨日界：23:50 -> 次日 00:10，前一日与次日各 10 分钟。
		{ID: "PASS-004", SatelliteID: "SAT-2402", StationID: "GS-KS",
			Start: ts(16, 23, 50), End: ts(17, 0, 10), MaxElevationDeg: 55.2},
		{ID: "PASS-005", SatelliteID: "SAT-2403", StationID: "GS-MOB",
			Start: ts(16, 8, 30), End: ts(16, 8, 39), MaxElevationDeg: 33.8},
		{ID: "PASS-006", SatelliteID: "SAT-2403", StationID: "GS-SY",
			Start: ts(17, 1, 15), End: ts(17, 1, 27), MaxElevationDeg: 58.9},
		// 跨日界：前一日 23:55 -> 基准日 00:05。
		{ID: "PASS-007", SatelliteID: "SAT-2401", StationID: "GS-SY",
			Start: ts(15, 23, 55), End: ts(16, 0, 5), MaxElevationDeg: 44.3},
	}
}

// CrossBoundaryPassIDs 返回跨越 UTC 日界的样例可见弧编号。
func CrossBoundaryPassIDs() []string {
	return []string{"PASS-004", "PASS-007"}
}

// Frames 返回样例遥测帧。
//
// FRAME-007 故意标记为损坏，用于演练入库失败上报；
// FRAME-012 长度明显偏大，用于演练归档段容量不变量。
func Frames() []model.Frame {
	mk := func(id, sat, pass string, minute, bytes int, corrupt bool) model.Frame {
		f := model.Frame{
			ID: id, SatelliteID: sat, PassID: pass,
			At: ts(16, 2, minute), Bytes: bytes, Corrupt: corrupt,
		}
		f.Checksum = f.ExpectedChecksum()
		if corrupt {
			// 损坏帧的校验和与期望值不符，用于演练入库失败上报。
			f.Checksum ^= 0x5A5A
		}
		return f
	}
	out := []model.Frame{
		mk("FRAME-001", "SAT-2401", "PASS-001", 10, 256, false),
		mk("FRAME-002", "SAT-2401", "PASS-001", 12, 256, false),
		mk("FRAME-003", "SAT-2401", "PASS-001", 14, 320, false),
		mk("FRAME-004", "SAT-2401", "PASS-002", 41, 256, false),
		mk("FRAME-005", "SAT-2402", "PASS-003", 6, 288, false),
		mk("FRAME-006", "SAT-2402", "PASS-003", 8, 288, false),
		mk("FRAME-007", "SAT-2402", "PASS-003", 10, 288, true),
		mk("FRAME-008", "SAT-2402", "PASS-004", 51, 256, false),
		mk("FRAME-009", "SAT-2403", "PASS-005", 31, 192, false),
		mk("FRAME-010", "SAT-2403", "PASS-005", 33, 192, false),
		mk("FRAME-011", "SAT-2401", "PASS-007", 1, 224, false),
		mk("FRAME-012", "SAT-2401", "PASS-007", 3, 4096, false),
	}
	return out
}

// CleanFrames 返回全部校验通过的样例遥测帧。
func CleanFrames() []model.Frame {
	out := make([]model.Frame, 0, len(Frames()))
	for _, f := range Frames() {
		if !f.Corrupt {
			out = append(out, f)
		}
	}
	return out
}

// CorruptFrameIDs 返回样例中已知损坏的帧编号。
func CorruptFrameIDs() []string {
	out := make([]string, 0, 1)
	for _, f := range Frames() {
		if f.Corrupt {
			out = append(out, f.ID)
		}
	}
	return out
}

// BandRequests 返回样例频轨申请，用于并发分配演练。
func BandRequests(n int) []model.Band {
	bands := []model.Band{model.BandKa, model.BandKu, model.BandQV}
	out := make([]model.Band, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, bands[i%len(bands)])
	}
	return out
}

// SatelliteIDFor 返回第 i 个并发申请所用的卫星编号。
func SatelliteIDFor(i int) string {
	return fmt.Sprintf("SAT-24%02d", i%4+1)
}
