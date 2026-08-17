// Package constellation 负责卫星与地面站登记及可测控性判定。
package constellation

import (
	"fmt"
	"sort"
	"sync"

	"satnet/internal/model"
)

// Registry 是星座与地面站台账。
type Registry struct {
	mu           sync.RWMutex
	sats         map[string]model.Satellite
	satOrder     []string
	stations     map[string]model.Station
	stationOrder []string
}

// NewRegistry 构造台账。
func NewRegistry() *Registry {
	return &Registry{
		sats:     make(map[string]model.Satellite),
		stations: make(map[string]model.Station),
	}
}

// AddSatellite 登记一颗卫星。
func (r *Registry) AddSatellite(s model.Satellite) error {
	if err := s.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.sats[s.ID]; !dup {
		r.satOrder = append(r.satOrder, s.ID)
	}
	r.sats[s.ID] = s
	return nil
}

// AddSatellites 批量登记卫星。
func (r *Registry) AddSatellites(items []model.Satellite) error {
	for _, s := range items {
		if err := r.AddSatellite(s); err != nil {
			return err
		}
	}
	return nil
}

// AddStation 登记一个地面站。
func (r *Registry) AddStation(s model.Station) error {
	if err := s.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.stations[s.ID]; !dup {
		r.stationOrder = append(r.stationOrder, s.ID)
	}
	r.stations[s.ID] = s
	return nil
}

// AddStations 批量登记地面站。
func (r *Registry) AddStations(items []model.Station) error {
	for _, s := range items {
		if err := r.AddStation(s); err != nil {
			return err
		}
	}
	return nil
}

// Satellite 按编号查询卫星。
func (r *Registry) Satellite(id string) (model.Satellite, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sats[id]
	if !ok {
		return model.Satellite{}, fmt.Errorf("%w: %s", model.ErrSatelliteNotFound, id)
	}
	return s, nil
}

// Station 按编号查询地面站。
func (r *Registry) Station(id string) (model.Station, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.stations[id]
	if !ok {
		return model.Station{}, fmt.Errorf("%w: %s", model.ErrStationNotFound, id)
	}
	return s, nil
}

// Satellites 按登记顺序返回全部卫星。
func (r *Registry) Satellites() []model.Satellite {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.Satellite, 0, len(r.sats))
	for _, id := range r.satOrder {
		out = append(out, r.sats[id])
	}
	return out
}

// Stations 按登记顺序返回全部地面站。
func (r *Registry) Stations() []model.Station {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.Station, 0, len(r.stations))
	for _, id := range r.stationOrder {
		out = append(out, r.stations[id])
	}
	return out
}

// TrackableSatellites 返回可安排测控窗口的卫星编号，按升序排列。
func (r *Registry) TrackableSatellites() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.sats))
	for id, s := range r.sats {
		if s.State.Trackable() {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// CheckTrackable 校验卫星是否可安排测控窗口。
func (r *Registry) CheckTrackable(satelliteID string) error {
	s, err := r.Satellite(satelliteID)
	if err != nil {
		return err
	}
	if !s.State.Trackable() {
		return fmt.Errorf("%w: 卫星 %s 处于%s",
			model.ErrSatelliteNotOperational, s.ID, s.State.DisplayName())
	}
	return nil
}

// CheckBand 校验卫星与地面站是否都支持给定频段。
func (r *Registry) CheckBand(satelliteID, stationID string, band model.Band) error {
	sat, err := r.Satellite(satelliteID)
	if err != nil {
		return err
	}
	st, err := r.Station(stationID)
	if err != nil {
		return err
	}
	if !sat.Supports(band) {
		return fmt.Errorf("%w: 卫星 %s 不支持%s", model.ErrInvalidGrant, sat.ID, band.DisplayName())
	}
	if !st.Supports(band) {
		return fmt.Errorf("%w: 地面站 %s 不支持%s", model.ErrInvalidGrant, st.ID, band.DisplayName())
	}
	return nil
}

// CountByState 按卫星状态统计数量。
func (r *Registry) CountByState() map[model.SatState]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[model.SatState]int)
	for _, s := range model.AllSatStates() {
		out[s] = 0
	}
	for _, s := range r.sats {
		out[s.State]++
	}
	return out
}

// Antennas 返回全部地面站的天线总数。
func (r *Registry) Antennas() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for _, s := range r.stations {
		n += s.Antennas
	}
	return n
}

// DescribeSatellite 返回卫星的单行描述。
func DescribeSatellite(s model.Satellite) string {
	return fmt.Sprintf("%s %s 轨道面%s %s 高度%.0fkm 倾角%.1f° 频段%v",
		s.ID, s.Name, s.Plane, s.State.DisplayName(), s.AltitudeKM, s.InclinationDeg, s.Bands)
}

// DescribeStation 返回地面站的单行描述。
func DescribeStation(s model.Station) string {
	return fmt.Sprintf("%s %s %s (%.2f, %.2f) 天线%d 频段%v",
		s.ID, s.Name, s.Kind.DisplayName(), s.LatDeg, s.LonDeg, s.Antennas, s.Bands)
}
