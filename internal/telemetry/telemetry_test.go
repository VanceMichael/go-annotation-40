package telemetry_test

import (
	"errors"
	"testing"

	"satnet/internal/model"
	"satnet/internal/seed"
	"satnet/internal/telemetry"
)

func TestVerifyAcceptsCleanFrames(t *testing.T) {
	for _, f := range seed.CleanFrames() {
		if err := telemetry.Verify(f); err != nil {
			t.Fatalf("帧 %s 应通过校验: %v", f.ID, err)
		}
	}
}

func TestVerifyRejectsCorruptFrame(t *testing.T) {
	ids := seed.CorruptFrameIDs()
	if len(ids) == 0 {
		t.Fatal("样例数据应包含至少一帧损坏帧")
	}
	for _, f := range seed.Frames() {
		if !f.Corrupt {
			continue
		}
		if err := telemetry.Verify(f); !errors.Is(err, model.ErrFrameChecksum) {
			t.Fatalf("损坏帧 %s 应返回 ErrFrameChecksum, 得到 %v", f.ID, err)
		}
	}
}

func TestIngestReportsChecksumFailure(t *testing.T) {
	s := telemetry.NewStore()
	n, err := s.Ingest(seed.Frames())
	if err == nil {
		t.Fatalf("批次中含损坏帧时必须上报失败，实际返回 nil（已入库 %d 帧）", n)
	}
	if n >= len(seed.Frames()) {
		t.Fatalf("入库帧数应少于提交帧数: %d / %d", n, len(seed.Frames()))
	}
	if s.Len() != n {
		t.Fatalf("台账帧数 %d 与返回入库数 %d 不一致", s.Len(), n)
	}
}

func TestIngestSurfacesErrorChain(t *testing.T) {
	s := telemetry.NewStore()
	_, err := s.Ingest(seed.Frames())
	if !errors.Is(err, model.ErrFrameChecksum) {
		t.Fatalf("错误链上应保留 ErrFrameChecksum, 得到 %v", err)
	}
}

func TestIngestSucceedsOnCleanBatch(t *testing.T) {
	s := telemetry.NewStore()
	clean := seed.CleanFrames()
	n, err := s.Ingest(clean)
	if err != nil {
		t.Fatalf("干净批次应当成功: %v", err)
	}
	if n != len(clean) || s.Len() != len(clean) {
		t.Fatalf("应全部入库: n=%d len=%d want=%d", n, s.Len(), len(clean))
	}
}

func TestIngestBatchReportsFailure(t *testing.T) {
	s := telemetry.NewStore()
	rep := s.IngestBatch(seed.Frames())
	if rep.NotIngested == 0 {
		t.Fatal("样例批次应存在未入库帧")
	}
	if !rep.Reported {
		t.Fatalf("存在未入库帧时必须上报失败: %+v", rep)
	}
	if rep.Message == "" {
		t.Fatal("上报失败时应带错误信息")
	}
}

func TestIngestBatchCleanIsSilent(t *testing.T) {
	s := telemetry.NewStore()
	rep := s.IngestBatch(seed.CleanFrames())
	if rep.NotIngested != 0 || rep.Reported {
		t.Fatalf("干净批次不应上报失败: %+v", rep)
	}
}

func TestIngestAllOrNothingRollsBack(t *testing.T) {
	s := telemetry.NewStore()
	if _, err := s.Ingest(seed.CleanFrames()[:2]); err != nil {
		t.Fatalf("预置入库失败: %v", err)
	}
	before := s.Len()
	n, err := s.IngestAllOrNothing(seed.Frames())
	if err == nil {
		t.Fatalf("含损坏帧的批次必须上报失败，实际返回 nil（入库 %d 帧）", n)
	}
	if n != 0 {
		t.Fatalf("整批回滚时入库数应为 0, 实际 %d", n)
	}
	if s.Len() != before {
		t.Fatalf("整批回滚后台账应保持 %d 帧, 实际 %d 帧", before, s.Len())
	}
}

func TestIngestStopsAtFirstFailure(t *testing.T) {
	s := telemetry.NewStore()
	n, _ := s.Ingest(seed.Frames())
	frames := seed.Frames()
	// 第一帧损坏帧之前的帧都应入库，之后的帧都不应入库。
	firstBad := -1
	for i, f := range frames {
		if f.Corrupt {
			firstBad = i
			break
		}
	}
	if firstBad < 0 {
		t.Fatal("样例数据应包含损坏帧")
	}
	if n != firstBad {
		t.Fatalf("应在第一帧损坏帧处停止: 入库 %d 帧, 损坏帧下标 %d", n, firstBad)
	}
}

func TestByPassCountsIngestedFrames(t *testing.T) {
	s := telemetry.NewStore()
	if _, err := s.Ingest(seed.CleanFrames()); err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	total := 0
	for _, v := range s.ByPass() {
		total += v
	}
	if total != s.Len() {
		t.Fatalf("按可见弧统计合计 %d 与台账 %d 不一致", total, s.Len())
	}
	if len(s.PassIDs()) == 0 {
		t.Fatal("应至少涉及一个可见弧")
	}
}

func TestBytesMatchesFrames(t *testing.T) {
	s := telemetry.NewStore()
	clean := seed.CleanFrames()
	if _, err := s.Ingest(clean); err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	want := 0
	for _, f := range clean {
		want += f.Bytes
	}
	if s.Bytes() != want {
		t.Fatalf("总字节数应为 %d, 实际 %d", want, s.Bytes())
	}
}
