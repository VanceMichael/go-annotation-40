// Package model 定义低轨卫星互联网星座测控与频轨资源调度平台的领域模型。
//
// 平台覆盖卫星与地面站登记、可见弧窗口计算、测控窗口切分、
// 频轨资源分配、遥测帧入库与归档。
package model

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Band 表示频段。
type Band string

const (
	// BandKu Ku 频段。
	BandKu Band = "ku"
	// BandKa Ka 频段。
	BandKa Band = "ka"
	// BandQV Q/V 频段。
	BandQV Band = "qv"
	// BandS S 频段，用于测控。
	BandS Band = "s"
)

// AllBands 返回全部频段。
func AllBands() []Band {
	return []Band{BandS, BandKu, BandKa, BandQV}
}

// DisplayName 返回频段中文名。
func (b Band) DisplayName() string {
	switch b {
	case BandKu:
		return "Ku频段"
	case BandKa:
		return "Ka频段"
	case BandQV:
		return "Q/V频段"
	case BandS:
		return "S频段"
	default:
		return string(b)
	}
}

// TotalMHz 返回该频段可分配的总带宽，单位兆赫。
func (b Band) TotalMHz() int {
	switch b {
	case BandS:
		return 20
	case BandKu:
		return 500
	case BandKa:
		return 1000
	case BandQV:
		return 2000
	default:
		return 0
	}
}

// ParseBand 解析频段代码。
func ParseBand(s string) (Band, error) {
	v := Band(strings.ToLower(strings.TrimSpace(s)))
	for _, b := range AllBands() {
		if v == b {
			return v, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownBand, s)
}

// SatState 表示卫星状态。
type SatState string

const (
	// SatLaunched 已发射，尚未入轨。
	SatLaunched SatState = "launched"
	// SatCommissioning 在轨测试中。
	SatCommissioning SatState = "commissioning"
	// SatOperational 在轨运行，可测控。
	SatOperational SatState = "operational"
	// SatSafeMode 安全模式，仅接受测控指令。
	SatSafeMode SatState = "safe-mode"
	// SatDeorbited 已离轨。
	SatDeorbited SatState = "deorbited"
)

// AllSatStates 返回全部卫星状态。
func AllSatStates() []SatState {
	return []SatState{SatLaunched, SatCommissioning, SatOperational, SatSafeMode, SatDeorbited}
}

// DisplayName 返回卫星状态中文名。
func (s SatState) DisplayName() string {
	switch s {
	case SatLaunched:
		return "已发射"
	case SatCommissioning:
		return "在轨测试"
	case SatOperational:
		return "在轨运行"
	case SatSafeMode:
		return "安全模式"
	case SatDeorbited:
		return "已离轨"
	default:
		return string(s)
	}
}

// Trackable 报告处于该状态的卫星是否可以安排测控窗口。
//
// 在轨运行与安全模式都可测控；已发射、在轨测试与已离轨不安排测控窗口。
func (s SatState) Trackable() bool {
	return s == SatOperational || s == SatSafeMode
}

// ParseSatState 解析卫星状态代码。
func ParseSatState(s string) (SatState, error) {
	v := SatState(strings.ToLower(strings.TrimSpace(s)))
	for _, k := range AllSatStates() {
		if v == k {
			return v, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownSatState, s)
}

// StationKind 表示地面站类型。
type StationKind string

const (
	// StationTTC 测控站。
	StationTTC StationKind = "ttc"
	// StationGateway 信关站。
	StationGateway StationKind = "gateway"
	// StationMobile 车载机动站。
	StationMobile StationKind = "mobile"
)

// AllStationKinds 返回全部地面站类型。
func AllStationKinds() []StationKind {
	return []StationKind{StationTTC, StationGateway, StationMobile}
}

// DisplayName 返回地面站类型中文名。
func (k StationKind) DisplayName() string {
	switch k {
	case StationTTC:
		return "测控站"
	case StationGateway:
		return "信关站"
	case StationMobile:
		return "车载机动站"
	default:
		return string(k)
	}
}

// ParseStationKind 解析地面站类型代码。
func ParseStationKind(s string) (StationKind, error) {
	v := StationKind(strings.ToLower(strings.TrimSpace(s)))
	for _, k := range AllStationKinds() {
		if v == k {
			return v, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownStationKind, s)
}

// Satellite 表示一颗在网卫星。
type Satellite struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Plane string   `json:"plane"`
	State SatState `json:"state"`
	// AltitudeKM 是标称轨道高度，单位千米。
	AltitudeKM float64 `json:"altitude_km"`
	// InclinationDeg 是轨道倾角，单位度。
	InclinationDeg float64 `json:"inclination_deg"`
	// Bands 是该卫星支持的频段。
	Bands []Band `json:"bands"`
}

// Validate 校验卫星登记。
func (s Satellite) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("%w: 卫星编号为空", ErrInvalidSatellite)
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("%w: 卫星 %s 缺少名称", ErrInvalidSatellite, s.ID)
	}
	if strings.TrimSpace(s.Plane) == "" {
		return fmt.Errorf("%w: 卫星 %s 缺少轨道面", ErrInvalidSatellite, s.ID)
	}
	if _, err := ParseSatState(string(s.State)); err != nil {
		return fmt.Errorf("%w: 卫星 %s 状态非法", ErrInvalidSatellite, s.ID)
	}
	if s.AltitudeKM <= 0 {
		return fmt.Errorf("%w: 卫星 %s 轨道高度必须为正", ErrInvalidSatellite, s.ID)
	}
	if s.InclinationDeg < 0 || s.InclinationDeg > 180 {
		return fmt.Errorf("%w: 卫星 %s 轨道倾角越界", ErrInvalidSatellite, s.ID)
	}
	if len(s.Bands) == 0 {
		return fmt.Errorf("%w: 卫星 %s 未登记任何频段", ErrInvalidSatellite, s.ID)
	}
	for _, b := range s.Bands {
		if _, err := ParseBand(string(b)); err != nil {
			return fmt.Errorf("%w: 卫星 %s 频段非法", ErrInvalidSatellite, s.ID)
		}
	}
	return nil
}

// Supports 报告该卫星是否支持给定频段。
func (s Satellite) Supports(b Band) bool {
	for _, v := range s.Bands {
		if v == b {
			return true
		}
	}
	return false
}

// Station 表示一个地面站。
type Station struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Kind StationKind `json:"kind"`
	// LatDeg 与 LonDeg 是站址纬经度，单位度。
	LatDeg float64 `json:"lat_deg"`
	LonDeg float64 `json:"lon_deg"`
	// Antennas 是可用天线数量，决定同时可跟踪的卫星数。
	Antennas int `json:"antennas"`
	// Bands 是该站支持的频段。
	Bands []Band `json:"bands"`
}

// Validate 校验地面站登记。
func (s Station) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("%w: 地面站编号为空", ErrInvalidStation)
	}
	if _, err := ParseStationKind(string(s.Kind)); err != nil {
		return fmt.Errorf("%w: 地面站 %s 类型非法", ErrInvalidStation, s.ID)
	}
	if s.LatDeg < -90 || s.LatDeg > 90 {
		return fmt.Errorf("%w: 地面站 %s 纬度越界", ErrInvalidStation, s.ID)
	}
	if s.LonDeg < -180 || s.LonDeg > 180 {
		return fmt.Errorf("%w: 地面站 %s 经度越界", ErrInvalidStation, s.ID)
	}
	if s.Antennas <= 0 {
		return fmt.Errorf("%w: 地面站 %s 天线数必须为正", ErrInvalidStation, s.ID)
	}
	if len(s.Bands) == 0 {
		return fmt.Errorf("%w: 地面站 %s 未登记任何频段", ErrInvalidStation, s.ID)
	}
	return nil
}

// Supports 报告该站是否支持给定频段。
func (s Station) Supports(b Band) bool {
	for _, v := range s.Bands {
		if v == b {
			return true
		}
	}
	return false
}

// Pass 表示一次可见弧，即卫星在某地面站上空的可跟踪时段。
type Pass struct {
	ID          string    `json:"id"`
	SatelliteID string    `json:"satellite_id"`
	StationID   string    `json:"station_id"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	// MaxElevationDeg 是最大仰角，单位度。
	MaxElevationDeg float64 `json:"max_elevation_deg"`
}

// Validate 校验可见弧。
func (p Pass) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("%w: 可见弧编号为空", ErrInvalidPass)
	}
	if strings.TrimSpace(p.SatelliteID) == "" || strings.TrimSpace(p.StationID) == "" {
		return fmt.Errorf("%w: 可见弧 %s 缺少卫星或地面站", ErrInvalidPass, p.ID)
	}
	if p.Start.IsZero() || p.End.IsZero() {
		return fmt.Errorf("%w: 可见弧 %s 缺少起止时间", ErrInvalidPass, p.ID)
	}
	if !p.End.After(p.Start) {
		return fmt.Errorf("%w: 可见弧 %s 结束时间必须晚于开始时间", ErrInvalidPass, p.ID)
	}
	if p.MaxElevationDeg <= 0 || p.MaxElevationDeg > 90 {
		return fmt.Errorf("%w: 可见弧 %s 最大仰角越界", ErrInvalidPass, p.ID)
	}
	return nil
}

// Duration 返回可见弧时长。
func (p Pass) Duration() time.Duration {
	return p.End.Sub(p.Start)
}

// Overlaps 报告两段可见弧在时间上是否重叠。
//
// 端点相接不算重叠：前一段的结束时刻等于后一段的开始时刻时互不冲突。
func (p Pass) Overlaps(o Pass) bool {
	return p.Start.Before(o.End) && o.Start.Before(p.End)
}

// Frame 表示一帧遥测。
type Frame struct {
	ID          string    `json:"id"`
	SatelliteID string    `json:"satellite_id"`
	PassID      string    `json:"pass_id"`
	At          time.Time `json:"at"`
	// Bytes 是帧长度，单位字节。
	Bytes int `json:"bytes"`
	// Checksum 是帧校验和。
	Checksum uint32 `json:"checksum"`
	// Corrupt 报告该帧是否已知损坏，仅用于本地演练。
	Corrupt bool `json:"corrupt"`
}

// Validate 校验遥测帧。
func (f Frame) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("%w: 遥测帧编号为空", ErrInvalidFrame)
	}
	if strings.TrimSpace(f.SatelliteID) == "" {
		return fmt.Errorf("%w: 遥测帧 %s 缺少卫星", ErrInvalidFrame, f.ID)
	}
	if f.Bytes <= 0 {
		return fmt.Errorf("%w: 遥测帧 %s 长度必须为正", ErrInvalidFrame, f.ID)
	}
	if f.At.IsZero() {
		return fmt.Errorf("%w: 遥测帧 %s 缺少时间戳", ErrInvalidFrame, f.ID)
	}
	return nil
}

// ExpectedChecksum 返回该帧按约定算出的校验和。
func (f Frame) ExpectedChecksum() uint32 {
	var sum uint32 = 2166136261
	for _, c := range []byte(f.ID + "|" + f.SatelliteID) {
		sum ^= uint32(c)
		sum *= 16777619
	}
	sum ^= uint32(f.Bytes)
	return sum
}

// SortPasses 按开始时间、卫星与编号排序，保证输出稳定。
func SortPasses(items []Pass) {
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].Start.Equal(items[j].Start) {
			return items[i].Start.Before(items[j].Start)
		}
		if items[i].SatelliteID != items[j].SatelliteID {
			return items[i].SatelliteID < items[j].SatelliteID
		}
		return items[i].ID < items[j].ID
	})
}

// UTCDay 返回该时刻所属的 UTC 自然日，格式 2006-01-02。
func UTCDay(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}
