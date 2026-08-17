package spectrum_test

import (
	"errors"
	"sync"
	"testing"

	"satnet/internal/model"
	"satnet/internal/spectrum"
)

func TestAllocateAndRelease(t *testing.T) {
	a := spectrum.NewAllocator()
	g, err := a.Allocate("SAT-2401", model.BandKa, 100)
	if err != nil {
		t.Fatalf("分配失败: %v", err)
	}
	if a.UsedMHz(model.BandKa) != 100 {
		t.Fatalf("已授权应为 100MHz, 实际 %d", a.UsedMHz(model.BandKa))
	}
	if err := a.Release(g.ID); err != nil {
		t.Fatalf("释放失败: %v", err)
	}
	if a.UsedMHz(model.BandKa) != 0 {
		t.Fatalf("释放后已授权应为 0, 实际 %d", a.UsedMHz(model.BandKa))
	}
	if err := a.Verify(); err != nil {
		t.Fatalf("释放后应自洽: %v", err)
	}
}

func TestAllocateRejectsWhenExhausted(t *testing.T) {
	a := spectrum.NewAllocator()
	total := model.BandS.TotalMHz()
	if _, err := a.Allocate("SAT-2401", model.BandS, total); err != nil {
		t.Fatalf("占满整段应当成功: %v", err)
	}
	_, err := a.Allocate("SAT-2402", model.BandS, 1)
	if !errors.Is(err, model.ErrBandExhausted) {
		t.Fatalf("带宽耗尽应返回 ErrBandExhausted, 得到 %v", err)
	}
	if a.Rejects() != 1 {
		t.Fatalf("拒绝次数应为 1, 实际 %d", a.Rejects())
	}
	if a.UsedMHz(model.BandS) != total {
		t.Fatalf("被拒申请不应改动已授权量: %d", a.UsedMHz(model.BandS))
	}
}

func TestAllocateRejectsBadInput(t *testing.T) {
	a := spectrum.NewAllocator()
	if _, err := a.Allocate("SAT-2401", model.Band("x"), 10); !errors.Is(err, model.ErrUnknownBand) {
		t.Fatalf("未知频段应返回 ErrUnknownBand, 得到 %v", err)
	}
	if _, err := a.Allocate("SAT-2401", model.BandKa, 0); !errors.Is(err, model.ErrInvalidGrant) {
		t.Fatalf("非正带宽应返回 ErrInvalidGrant, 得到 %v", err)
	}
}

// concurrentAllocate 并发申请并返回成功次数。
func concurrentAllocate(a *spectrum.Allocator, band model.Band, workers, perWorker, mhz int) int {
	var wg sync.WaitGroup
	var mu sync.Mutex
	granted := 0
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				if _, err := a.Allocate("SAT-2401", band, mhz); err == nil {
					mu.Lock()
					granted++
					mu.Unlock()
				}
			}
		}(w)
	}
	wg.Wait()
	return granted
}

func TestConcurrentAllocateKeepsGrantsConsistent(t *testing.T) {
	// 重复多轮，每轮用全新的分配器：任意一轮出现丢失即为不合格。
	const rounds = 20
	for r := 1; r <= rounds; r++ {
		a := spectrum.NewAllocator()
		granted := concurrentAllocate(a, model.BandQV, 64, 5, 5)
		if got := len(a.Grants()); got != granted {
			t.Fatalf("第 %d/%d 轮: 成功申请 %d 次, 授权记录只有 %d 条（丢失 %d 条）",
				r, rounds, granted, got, granted-got)
		}
		if want, got := granted*5, a.UsedMHz(model.BandQV); want != got {
			t.Fatalf("第 %d/%d 轮: 已授权带宽应为 %dMHz, 实际 %dMHz", r, rounds, want, got)
		}
		if err := a.Verify(); err != nil {
			t.Fatalf("第 %d/%d 轮核对失败: %v", r, rounds, err)
		}
	}
}

func TestConcurrentAllocateNeverOversubscribes(t *testing.T) {
	a := spectrum.NewAllocator()
	// 申请总量刻意超过 Ku 频段总带宽。
	_ = concurrentAllocate(a, model.BandKu, 64, 4, 10)
	used := a.UsedMHz(model.BandKu)
	if used > model.BandKu.TotalMHz() {
		t.Fatalf("已授权 %dMHz 超过总带宽 %dMHz", used, model.BandKu.TotalMHz())
	}
	if err := a.Verify(); err != nil {
		t.Fatalf("并发分配后应自洽: %v", err)
	}
}

func TestAllocateVerifyAfterConcurrentLoad(t *testing.T) {
	a := spectrum.NewAllocator()
	granted := concurrentAllocate(a, model.BandKa, 32, 8, 2)
	if err := a.Verify(); err != nil {
		t.Fatalf("并发分配后核对失败: %v", err)
	}
	if got := a.GrantedMHz(model.BandKa); got != granted*2 {
		t.Fatalf("授权记录累加应为 %dMHz, 实际 %dMHz", granted*2, got)
	}
}

func TestSnapshotCoversAllBands(t *testing.T) {
	a := spectrum.NewAllocator()
	if _, err := a.Allocate("SAT-2401", model.BandKa, 40); err != nil {
		t.Fatalf("分配失败: %v", err)
	}
	snap := a.Snapshot()
	if len(snap) != len(model.AllBands()) {
		t.Fatalf("快照应覆盖全部频段: %d", len(snap))
	}
	for _, u := range snap {
		if u.UsedMHz != u.GrantedMHz {
			t.Fatalf("频段 %s 已授权 %d 与记录 %d 不一致", u.Band, u.UsedMHz, u.GrantedMHz)
		}
	}
}

func TestReleaseUnknownGrant(t *testing.T) {
	a := spectrum.NewAllocator()
	if err := a.Release("G-NOPE"); !errors.Is(err, model.ErrInvalidGrant) {
		t.Fatalf("释放不存在的授权应返回 ErrInvalidGrant, 得到 %v", err)
	}
}
