// Package archive 负责把已入库遥测帧写出为归档段。
package archive

import (
	"fmt"
	"sort"

	"satnet/internal/model"
)

// Segment 是一个归档段。
type Segment struct {
	ID string `json:"id"`
	// SatelliteID 是该段所属卫星。
	SatelliteID string `json:"satellite_id"`
	// Frames 是段内帧数。
	Frames int `json:"frames"`
	// Bytes 是段内总字节数。
	Bytes int `json:"bytes"`
	// FrameIDs 是段内帧编号。
	FrameIDs []string `json:"frame_ids"`
}

// Archiver 把遥测帧切分成归档段。
type Archiver struct {
	// MaxFramesPerSegment 是单个归档段允许的最大帧数。
	MaxFramesPerSegment int
	// MaxBytesPerSegment 是单个归档段允许的最大字节数。
	MaxBytesPerSegment int
}

// New 构造归档器。
func New(maxFrames, maxBytes int) *Archiver {
	return &Archiver{MaxFramesPerSegment: maxFrames, MaxBytesPerSegment: maxBytes}
}

// mustFitSegment 校验段内不变量。
//
// 段容量是归档格式的硬约束，被突破意味着上游切分逻辑已经失效，
// 属于内部不变量破坏，因此以 panic 上抛，由 Flush 统一转换为错误返回。
func (a *Archiver) mustFitSegment(index, frames, bytes int) {
	if frames > a.MaxFramesPerSegment {
		panic(fmt.Sprintf("归档段 %d 帧数 %d 超过上限 %d", index, frames, a.MaxFramesPerSegment))
	}
	if bytes > a.MaxBytesPerSegment {
		panic(fmt.Sprintf("归档段 %d 字节数 %d 超过上限 %d", index, bytes, a.MaxBytesPerSegment))
	}
}

// Flush 把遥测帧写出为归档段。
//
// 内部不变量被破坏时以 panic 上抛，本函数负责统一转换为
// model.ErrArchiveWrite 错误返回给调用方；
// 不得把归档失败当成「本批没有可归档数据」的空归档静默返回。
func (a *Archiver) Flush(frames []model.Frame) (segs []Segment, err error) {
	defer func() {
		if r := recover(); r != nil {
			segs = nil
			err = fmt.Errorf("%w: 归档中断: %v", model.ErrArchiveWrite, r)
		}
	}()

	if a.MaxFramesPerSegment <= 0 || a.MaxBytesPerSegment <= 0 {
		return nil, fmt.Errorf("%w: 归档段容量必须为正", model.ErrArchiveWrite)
	}

	bySat := make(map[string][]model.Frame)
	for _, f := range frames {
		bySat[f.SatelliteID] = append(bySat[f.SatelliteID], f)
	}
	sats := make([]string, 0, len(bySat))
	for s := range bySat {
		sats = append(sats, s)
	}
	sort.Strings(sats)

	out := make([]Segment, 0, len(frames))
	index := 0
	for _, sat := range sats {
		group := bySat[sat]
		sort.SliceStable(group, func(i, j int) bool { return group[i].ID < group[j].ID })

		cur := Segment{SatelliteID: sat}
		flushCur := func() {
			if cur.Frames == 0 {
				return
			}
			index++
			cur.ID = fmt.Sprintf("SEG-%s-%03d", sat, index)
			a.mustFitSegment(index, cur.Frames, cur.Bytes)
			out = append(out, cur)
			cur = Segment{SatelliteID: sat}
		}

		for _, f := range group {
			if cur.Frames+1 > a.MaxFramesPerSegment ||
				cur.Bytes+f.Bytes > a.MaxBytesPerSegment {
				flushCur()
			}
			cur.Frames++
			cur.Bytes += f.Bytes
			cur.FrameIDs = append(cur.FrameIDs, f.ID)
		}
		flushCur()
	}
	return out, nil
}

// TotalFrames 返回全部归档段内的帧数。
func TotalFrames(segs []Segment) int {
	n := 0
	for _, s := range segs {
		n += s.Frames
	}
	return n
}

// TotalBytes 返回全部归档段内的字节数。
func TotalBytes(segs []Segment) int {
	n := 0
	for _, s := range segs {
		n += s.Bytes
	}
	return n
}

// Verify 交叉核对归档段与入库明细。
//
// 归档段内的帧数与字节数必须与入库明细完全一致。
func Verify(segs []Segment, frames []model.Frame) error {
	wantFrames := len(frames)
	wantBytes := 0
	for _, f := range frames {
		wantBytes += f.Bytes
	}
	if got := TotalFrames(segs); got != wantFrames {
		return fmt.Errorf("%w: 归档 %d 帧, 入库明细 %d 帧",
			model.ErrArchiveIncomplete, got, wantFrames)
	}
	if got := TotalBytes(segs); got != wantBytes {
		return fmt.Errorf("%w: 归档 %d 字节, 入库明细 %d 字节",
			model.ErrArchiveIncomplete, got, wantBytes)
	}
	return nil
}

// Describe 返回归档段的单行描述。
func Describe(s Segment) string {
	return fmt.Sprintf("%s %s %d帧 %d字节", s.ID, s.SatelliteID, s.Frames, s.Bytes)
}
