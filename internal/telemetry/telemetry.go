// Package telemetry 负责遥测帧的校验与入库。
package telemetry

import (
	"fmt"
	"sort"

	"satnet/internal/model"
)

// Verify 校验单帧遥测。
//
// 帧自身非法时返回 model.ErrInvalidFrame；校验和不符时返回 model.ErrFrameChecksum。
func Verify(f model.Frame) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if f.Corrupt || f.Checksum != f.ExpectedChecksum() {
		return fmt.Errorf("%w: 帧 %s 校验和 %d, 期望 %d",
			model.ErrFrameChecksum, f.ID, f.Checksum, f.ExpectedChecksum())
	}
	return nil
}

// Store 是遥测帧台账。
type Store struct {
	frames  []model.Frame
	rejects int
}

// NewStore 构造遥测帧台账。
func NewStore() *Store {
	return &Store{}
}

// Len 返回已入库帧数。
func (s *Store) Len() int {
	return len(s.frames)
}

// Rejects 返回被拒绝的帧数。
func (s *Store) Rejects() int {
	return s.rejects
}

// Frames 返回已入库帧的独立副本。
func (s *Store) Frames() []model.Frame {
	out := make([]model.Frame, len(s.frames))
	copy(out, s.frames)
	return out
}

// Bytes 返回已入库帧的总字节数。
func (s *Store) Bytes() int {
	n := 0
	for _, f := range s.frames {
		n += f.Bytes
	}
	return n
}

// Ingest 按顺序入库一批遥测帧。
//
// 遇到第一帧校验失败时停止入库，并把该失败通过返回值上报给调用方：
// 返回值 n 是本次成功入库的帧数，err 必须非 nil 且错误链上保留原始校验错误。
// 不得在有帧校验失败的情况下返回 nil 错误。
func (s *Store) Ingest(frames []model.Frame) (n int, err error) {
	for _, f := range frames {
		if verr := Verify(f); verr != nil {
			s.rejects++
			err = fmt.Errorf("入库遥测帧 %s 失败: %w", f.ID, verr)
			break
		}
		s.frames = append(s.frames, f)
		n++
	}
	return n, err
}

// IngestAllOrNothing 入库一批帧，任一帧失败时整批回滚。
func (s *Store) IngestAllOrNothing(frames []model.Frame) (int, error) {
	before := len(s.frames)
	n, err := s.Ingest(frames)
	if err != nil {
		s.frames = s.frames[:before:before]
		return 0, err
	}
	return n, nil
}

// ByPass 按可见弧分组统计已入库帧数。
func (s *Store) ByPass() map[string]int {
	out := make(map[string]int)
	for _, f := range s.frames {
		out[f.PassID]++
	}
	return out
}

// PassIDs 返回已入库帧涉及的可见弧编号，按升序排列。
func (s *Store) PassIDs() []string {
	set := s.ByPass()
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Report 是一次入库的结果摘要。
type Report struct {
	// Submitted 是提交的帧数。
	Submitted int `json:"submitted"`
	// Ingested 是成功入库的帧数。
	Ingested int `json:"ingested"`
	// NotIngested 是未能入库的帧数，等于 Submitted 减 Ingested。
	NotIngested int `json:"not_ingested"`
	// Reported 报告本次入库是否把失败上报给了调用方。
	Reported bool `json:"reported"`
	// Message 是上报的错误信息，成功时为空。
	Message string `json:"message,omitempty"`
}

// IngestBatch 入库一批帧并生成结果摘要。
func (s *Store) IngestBatch(frames []model.Frame) Report {
	before := s.Len()
	n, err := s.Ingest(frames)
	rep := Report{
		Submitted:   len(frames),
		Ingested:    n,
		NotIngested: len(frames) - n,
		Reported:    err != nil,
	}
	if err != nil {
		rep.Message = err.Error()
	}
	// 已入库帧数必须与返回的入库计数一致。
	if s.Len()-before != n {
		rep.Message = fmt.Sprintf("入库计数 %d 与台账增量 %d 不一致", n, s.Len()-before)
	}
	return rep
}

// Describe 返回遥测帧的单行描述。
func Describe(f model.Frame) string {
	return fmt.Sprintf("%s %s@%s %d字节 校验和%d",
		f.ID, f.SatelliteID, f.PassID, f.Bytes, f.Checksum)
}
