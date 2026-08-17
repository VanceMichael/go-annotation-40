package window_test

import (
	"testing"
	"time"

	"satnet/internal/seed"
	"satnet/internal/window"
)

func plan() []window.Slot {
	return window.SplitAll(seed.Passes(), 5*time.Minute, 120)
}

func cloneSlots(items []window.Slot) []window.Slot {
	out := make([]window.Slot, len(items))
	copy(out, items)
	return out
}

func equalSlots(a, b []window.Slot) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSplitProducesSlots(t *testing.T) {
	slots := plan()
	if len(slots) == 0 {
		t.Fatal("样例可见弧应当切出时隙")
	}
	for _, s := range slots {
		if s.Duration() <= 0 {
			t.Fatalf("时隙 %s 时长必须为正", s.ID)
		}
	}
}

func TestGroupsCoverEverySlot(t *testing.T) {
	slots := plan()
	groups := window.Groups(slots)
	if got := window.TotalSlots(groups); got != len(slots) {
		t.Fatalf("分组应覆盖全部时隙: %d / %d", got, len(slots))
	}
	if err := window.Verify(groups); err != nil {
		t.Fatalf("分组应自洽: %v", err)
	}
}

func TestGroupsDoNotMutateInput(t *testing.T) {
	slots := plan()
	before := cloneSlots(slots)
	_ = window.Groups(slots)
	if !equalSlots(before, slots) {
		t.Fatal("分组不应改动传入的时隙清单")
	}
}

func TestGroupsAreIndependent(t *testing.T) {
	slots := plan()
	groups := window.Groups(slots)
	ids := window.SatelliteIDs(groups)
	if len(ids) < 2 {
		t.Fatalf("样例数据应涉及至少 2 颗卫星, 实际 %v", ids)
	}

	// 记录各组首个时隙，随后只向第一组追加。
	firstBefore := make(map[string]string)
	for _, id := range ids {
		firstBefore[id] = groups[id][0].ID
	}

	target := ids[0]
	window.Extend(groups, target, window.Slot{
		ID: "EXTRA-W01", SatelliteID: target, StationID: "GS-KS",
		PassID: "PASS-EXTRA", Start: seed.Now(), End: seed.Now().Add(time.Minute),
	})

	for _, id := range ids {
		if id == target {
			continue
		}
		if got := groups[id][0].ID; got != firstBefore[id] {
			t.Fatalf("向 %s 追加时隙后，%s 的首个时隙从 %s 变成了 %s",
				target, id, firstBefore[id], got)
		}
	}
}

func TestExtendDoesNotTouchOtherGroups(t *testing.T) {
	slots := plan()
	groups := window.Groups(slots)
	before := window.TotalSlots(groups)

	ids := window.SatelliteIDs(groups)
	target := ids[0]
	window.Extend(groups, target, window.Slot{
		ID: "EXTRA-W02", SatelliteID: target, StationID: "GS-SY",
		PassID: "PASS-EXTRA", Start: seed.Now(), End: seed.Now().Add(2 * time.Minute),
	})

	if got := window.TotalSlots(groups); got != before+1 {
		t.Fatalf("追加后时隙总数应为 %d, 实际 %d", before+1, got)
	}
	if got := window.Misassigned(groups); got != 0 {
		t.Fatalf("追加后不应有归属错误的时隙, 实际 %d 个", got)
	}
}

func TestVerifyAfterExtend(t *testing.T) {
	groups := window.Groups(plan())
	ids := window.SatelliteIDs(groups)
	window.Extend(groups, ids[0], window.Slot{
		ID: "EXTRA-W03", SatelliteID: ids[0], StationID: "GS-MH",
		PassID: "PASS-EXTRA", Start: seed.Now(), End: seed.Now().Add(3 * time.Minute),
	})
	if err := window.Verify(groups); err != nil {
		t.Fatalf("追加后分组应仍然自洽: %v", err)
	}
	if got, want := window.DistinctSlotIDs(groups), window.TotalSlots(groups); got != want {
		t.Fatalf("互异时隙编号 %d 应等于时隙总数 %d", got, want)
	}
}

func TestExtendRepeatedlyKeepsConsistency(t *testing.T) {
	groups := window.Groups(plan())
	ids := window.SatelliteIDs(groups)
	for i := 0; i < 3; i++ {
		window.Extend(groups, ids[0], window.Slot{
			ID:          "EXTRA-R" + string(rune('A'+i)),
			SatelliteID: ids[0], StationID: "GS-KS", PassID: "PASS-EXTRA",
			Start: seed.Now(), End: seed.Now().Add(time.Minute),
		})
		if err := window.Verify(groups); err != nil {
			t.Fatalf("第 %d 次追加后分组应自洽: %v", i+1, err)
		}
	}
}

func TestSplitDropsShortTail(t *testing.T) {
	passes := seed.Passes()
	// 用一个很长的时隙长度，使尾段不足最小时长而被丢弃。
	slots := window.SplitAll(passes, 30*time.Minute, 1200)
	for _, s := range slots {
		if int(s.Duration()/time.Second) < 1200 {
			t.Fatalf("时隙 %s 时长 %v 低于最小时长", s.ID, s.Duration())
		}
	}
}

func TestSatelliteIDsSorted(t *testing.T) {
	ids := window.SatelliteIDs(window.Groups(plan()))
	for i := 1; i < len(ids); i++ {
		if ids[i-1] > ids[i] {
			t.Fatalf("卫星编号应升序: %v", ids)
		}
	}
}
