// Package window 负责把可见弧切分成测控时隙并按卫星分组。
package window

import (
	"fmt"
	"sort"
	"time"

	"satnet/internal/model"
)

// Slot 是一个测控时隙。
type Slot struct {
	ID          string    `json:"id"`
	SatelliteID string    `json:"satellite_id"`
	StationID   string    `json:"station_id"`
	PassID      string    `json:"pass_id"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
}

// Duration 返回时隙时长。
func (s Slot) Duration() time.Duration {
	return s.End.Sub(s.Start)
}

// Split 把一段可见弧按固定长度切成测控时隙。
//
// 不足一个完整长度的尾段仍然保留，但不足 minSeconds 秒的尾段被丢弃。
func Split(p model.Pass, slot time.Duration, minSeconds int) []Slot {
	out := make([]Slot, 0, 4)
	if slot <= 0 {
		return out
	}
	n := 0
	for cur := p.Start; cur.Before(p.End); cur = cur.Add(slot) {
		end := cur.Add(slot)
		if end.After(p.End) {
			end = p.End
		}
		if int(end.Sub(cur)/time.Second) < minSeconds {
			break
		}
		n++
		out = append(out, Slot{
			ID:          fmt.Sprintf("%s-W%02d", p.ID, n),
			SatelliteID: p.SatelliteID,
			StationID:   p.StationID,
			PassID:      p.ID,
			Start:       cur,
			End:         end,
		})
	}
	return out
}

// SplitAll 把多段可见弧切成时隙。
func SplitAll(passes []model.Pass, slot time.Duration, minSeconds int) []Slot {
	out := make([]Slot, 0, len(passes)*4)
	for _, p := range passes {
		out = append(out, Split(p, slot, minSeconds)...)
	}
	return out
}

// SortSlots 按卫星、开始时间与编号排序。
func SortSlots(items []Slot) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SatelliteID != items[j].SatelliteID {
			return items[i].SatelliteID < items[j].SatelliteID
		}
		if !items[i].Start.Equal(items[j].Start) {
			return items[i].Start.Before(items[j].Start)
		}
		return items[i].ID < items[j].ID
	})
}

// Groups 按卫星把测控时隙分组。
//
// 每一组都持有独立存储：对任意一组追加时隙都不会影响其他组，
// 也不会改动传入的时隙清单。
func Groups(slots []Slot) map[string][]Slot {
	out := make(map[string][]Slot)
	for _, s := range slots {
		out[s.SatelliteID] = append(out[s.SatelliteID], s)
	}
	for k := range out {
		SortSlots(out[k])
	}
	return out
}

// SatelliteIDs 返回分组中的卫星编号，按升序排列。
func SatelliteIDs(groups map[string][]Slot) []string {
	out := make([]string, 0, len(groups))
	for k := range groups {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Extend 给某颗卫星的时隙组追加一个时隙。
//
// 追加只影响该组：其他组的时隙内容与顺序都不变。
func Extend(groups map[string][]Slot, satelliteID string, extra Slot) {
	groups[satelliteID] = append(groups[satelliteID], extra)
	SortSlots(groups[satelliteID])
}

// TotalSlots 返回分组中的时隙总数。
func TotalSlots(groups map[string][]Slot) int {
	n := 0
	for _, v := range groups {
		n += len(v)
	}
	return n
}

// Misassigned 返回归属错误的时隙数量，即出现在别的卫星分组里的时隙。
func Misassigned(groups map[string][]Slot) int {
	n := 0
	for satID, slots := range groups {
		for _, s := range slots {
			if s.SatelliteID != satID {
				n++
			}
		}
	}
	return n
}

// DistinctSlotIDs 返回分组中互不相同的时隙编号数量。
func DistinctSlotIDs(groups map[string][]Slot) int {
	seen := make(map[string]struct{})
	for _, slots := range groups {
		for _, s := range slots {
			seen[s.ID] = struct{}{}
		}
	}
	return len(seen)
}

// Verify 校验分组的自洽性。
//
// 每个时隙都必须出现在与自身卫星编号一致的分组里，且时隙编号互不相同。
func Verify(groups map[string][]Slot) error {
	if n := Misassigned(groups); n > 0 {
		return fmt.Errorf("%w: %d 个时隙归属到了别的卫星", model.ErrWindowSliceAliased, n)
	}
	if got, want := DistinctSlotIDs(groups), TotalSlots(groups); got != want {
		return fmt.Errorf("%w: 互异时隙编号 %d 个, 时隙总数 %d 个",
			model.ErrWindowSliceAliased, got, want)
	}
	return nil
}

// Describe 返回时隙的单行描述。
func Describe(s Slot) string {
	return fmt.Sprintf("%s %s@%s %s -> %s (%.0f秒)",
		s.ID, s.SatelliteID, s.StationID,
		s.Start.UTC().Format("15:04:05"), s.End.UTC().Format("15:04:05"),
		s.Duration().Seconds())
}
