// Package pass 负责可见弧的裁剪、按自然日归集与冲突判定。
package pass

import (
	"fmt"
	"sort"
	"time"

	"satnet/internal/model"
)

// ParseDay 解析 2006-01-02 形式的 UTC 自然日，返回该日零时。
func ParseDay(day string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", day, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: 自然日 %q 无法解析", model.ErrInvalidPass, day)
	}
	return t, nil
}

// ClipToDay 把可见弧裁剪到给定 UTC 自然日之内。
//
// 跨 UTC 日界的可见弧会被裁成该自然日内的片段，而不是整段归给某一天：
// 一段 23:50 延续到次日 00:10 的可见弧，在前一日贡献 10 分钟、在次日也贡献 10 分钟。
// 与该自然日完全不相交时返回 false。
func ClipToDay(p model.Pass, day string) (model.Pass, bool) {
	dayStart, err := ParseDay(day)
	if err != nil {
		return model.Pass{}, false
	}
	dayEnd := dayStart.Add(24 * time.Hour)

	if !p.Start.Before(dayEnd) || !dayStart.Before(p.End) {
		return model.Pass{}, false
	}

	out := p
	if out.Start.Before(dayStart) {
		out.Start = dayStart
	}
	if out.End.After(dayEnd) {
		out.End = dayEnd
	}
	return out, true
}

// OnDay 返回落在给定 UTC 自然日内的可见弧片段。
func OnDay(passes []model.Pass, day string) []model.Pass {
	out := make([]model.Pass, 0, len(passes))
	for _, p := range passes {
		if clipped, ok := ClipToDay(p, day); ok {
			out = append(out, clipped)
		}
	}
	model.SortPasses(out)
	return out
}

// CoverageOnDay 返回给定自然日内可见弧的累计时长。
func CoverageOnDay(passes []model.Pass, day string) time.Duration {
	var total time.Duration
	for _, p := range OnDay(passes, day) {
		total += p.Duration()
	}
	return total
}

// SpanningDays 返回可见弧跨越的全部 UTC 自然日，按升序排列。
func SpanningDays(p model.Pass) []string {
	days := make([]string, 0, 2)
	cur := time.Date(p.Start.UTC().Year(), p.Start.UTC().Month(), p.Start.UTC().Day(),
		0, 0, 0, 0, time.UTC)
	for cur.Before(p.End) {
		days = append(days, cur.Format("2006-01-02"))
		cur = cur.Add(24 * time.Hour)
	}
	return days
}

// CrossesDayBoundary 报告该可见弧是否跨越 UTC 日界。
func CrossesDayBoundary(p model.Pass) bool {
	return len(SpanningDays(p)) > 1
}

// DayCoverage 汇总每个自然日的可见弧时长与段数。
type DayCoverage struct {
	Day string `json:"day"`
	// Passes 是该自然日内的可见弧片段数。
	Passes int `json:"passes"`
	// SecondsTotal 是该自然日内可见弧累计秒数。
	SecondsTotal int64 `json:"seconds_total"`
}

// CoverageByDay 按自然日汇总可见弧覆盖情况。
func CoverageByDay(passes []model.Pass, days []string) []DayCoverage {
	out := make([]DayCoverage, 0, len(days))
	for _, d := range days {
		clipped := OnDay(passes, d)
		var secs int64
		for _, p := range clipped {
			secs += int64(p.Duration() / time.Second)
		}
		out = append(out, DayCoverage{Day: d, Passes: len(clipped), SecondsTotal: secs})
	}
	return out
}

// TotalSeconds 返回全部可见弧的累计秒数，跨日界的可见弧只计一次。
func TotalSeconds(passes []model.Pass) int64 {
	var secs int64
	for _, p := range passes {
		secs += int64(p.Duration() / time.Second)
	}
	return secs
}

// Conflicts 返回同一地面站上相互重叠的可见弧对数。
//
// 端点相接不算冲突。
func Conflicts(passes []model.Pass) int {
	byStation := make(map[string][]model.Pass)
	for _, p := range passes {
		byStation[p.StationID] = append(byStation[p.StationID], p)
	}
	stations := make([]string, 0, len(byStation))
	for s := range byStation {
		stations = append(stations, s)
	}
	sort.Strings(stations)

	n := 0
	for _, s := range stations {
		group := byStation[s]
		model.SortPasses(group)
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				if group[i].Overlaps(group[j]) {
					n++
				}
			}
		}
	}
	return n
}

// Describe 返回可见弧的单行描述。
func Describe(p model.Pass) string {
	return fmt.Sprintf("%s %s@%s %s -> %s 时长%.0f秒 最大仰角%.1f°",
		p.ID, p.SatelliteID, p.StationID,
		p.Start.UTC().Format("2006-01-02 15:04:05"),
		p.End.UTC().Format("2006-01-02 15:04:05"),
		p.Duration().Seconds(), p.MaxElevationDeg)
}
