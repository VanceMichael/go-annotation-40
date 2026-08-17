package archive_test

import (
	"errors"
	"testing"

	"satnet/internal/archive"
	"satnet/internal/model"
	"satnet/internal/seed"
)

func frames(t *testing.T) []model.Frame {
	t.Helper()
	return seed.CleanFrames()
}

func maxBytes(fs []model.Frame) int {
	m := 0
	for _, f := range fs {
		if f.Bytes > m {
			m = f.Bytes
		}
	}
	return m
}

func TestFlushNormalPathMatchesFrames(t *testing.T) {
	fs := frames(t)
	segs, err := archive.New(4, 8192).Flush(fs)
	if err != nil {
		t.Fatalf("正常归档应当成功: %v", err)
	}
	if len(segs) == 0 {
		t.Fatal("应产出归档段")
	}
	if err := archive.Verify(segs, fs); err != nil {
		t.Fatalf("归档应与入库明细一致: %v", err)
	}
}

func TestFlushRespectsSegmentLimits(t *testing.T) {
	fs := frames(t)
	segs, err := archive.New(2, 8192).Flush(fs)
	if err != nil {
		t.Fatalf("归档失败: %v", err)
	}
	for _, s := range segs {
		if s.Frames > 2 {
			t.Fatalf("段 %s 帧数 %d 超过上限 2", s.ID, s.Frames)
		}
	}
	if err := archive.Verify(segs, fs); err != nil {
		t.Fatalf("归档应与入库明细一致: %v", err)
	}
}

func TestFlushReportsSegmentInvariantBreach(t *testing.T) {
	fs := frames(t)
	limit := maxBytes(fs) - 1
	if limit <= 0 {
		t.Fatal("样例数据应包含正长度的帧")
	}
	// 单段字节上限小于最大帧长，段容量不变量必然被突破。
	_, err := archive.New(4, limit).Flush(fs)
	if err == nil {
		t.Fatal("段容量不变量被突破时必须上报失败，实际返回 nil")
	}
	if !errors.Is(err, model.ErrArchiveWrite) {
		t.Fatalf("应返回 ErrArchiveWrite, 得到 %v", err)
	}
}

func TestFlushDoesNotReturnEmptyArchiveOnFailure(t *testing.T) {
	fs := frames(t)
	limit := maxBytes(fs) - 1
	segs, err := archive.New(4, limit).Flush(fs)
	if err == nil {
		t.Fatalf("归档失败时必须上报错误，实际返回 %d 个段且 err 为 nil", len(segs))
	}
	// 失败时不得把「零个归档段 + nil 错误」当成本批没有可归档数据。
	if verr := archive.Verify(segs, fs); verr == nil {
		t.Fatal("失败后的归档结果不应与入库明细一致")
	}
}

func TestFlushRejectsNonPositiveLimits(t *testing.T) {
	fs := frames(t)
	if _, err := archive.New(0, 1024).Flush(fs); !errors.Is(err, model.ErrArchiveWrite) {
		t.Fatalf("帧数上限为 0 应返回 ErrArchiveWrite, 得到 %v", err)
	}
	if _, err := archive.New(4, 0).Flush(fs); !errors.Is(err, model.ErrArchiveWrite) {
		t.Fatalf("字节上限为 0 应返回 ErrArchiveWrite, 得到 %v", err)
	}
}

func TestVerifyDetectsMismatch(t *testing.T) {
	fs := frames(t)
	segs, err := archive.New(4, 8192).Flush(fs)
	if err != nil {
		t.Fatalf("归档失败: %v", err)
	}
	if verr := archive.Verify(segs, fs[:len(fs)-1]); !errors.Is(verr, model.ErrArchiveIncomplete) {
		t.Fatalf("明细不一致应返回 ErrArchiveIncomplete, 得到 %v", verr)
	}
}

func TestFlushEmptyInput(t *testing.T) {
	segs, err := archive.New(4, 8192).Flush(nil)
	if err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
	if len(segs) != 0 {
		t.Fatalf("空输入应产出 0 个段, 实际 %d", len(segs))
	}
}

func TestFlushGroupsBySatellite(t *testing.T) {
	fs := frames(t)
	segs, err := archive.New(4, 8192).Flush(fs)
	if err != nil {
		t.Fatalf("归档失败: %v", err)
	}
	for _, s := range segs {
		if s.SatelliteID == "" {
			t.Fatalf("段 %s 缺少卫星编号", s.ID)
		}
		if len(s.FrameIDs) != s.Frames {
			t.Fatalf("段 %s 帧编号数 %d 与帧数 %d 不一致", s.ID, len(s.FrameIDs), s.Frames)
		}
	}
}
