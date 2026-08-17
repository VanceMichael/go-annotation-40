// Package cli 实现 satctl 命令行。
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"satnet/internal/archive"
	"satnet/internal/constellation"
	"satnet/internal/httpapi"
	"satnet/internal/model"
	"satnet/internal/pass"
	"satnet/internal/report"
	"satnet/internal/seed"
	"satnet/internal/spectrum"
	"satnet/internal/telemetry"
	"satnet/internal/window"
)

// 退出码约定，与 README 保持一致。
const (
	// ExitOK 成功。
	ExitOK = 0
	// ExitUsage 用法错误或未归类的内部错误。
	ExitUsage = 1
	// ExitInvalidArg 参数非法。
	ExitInvalidArg = 2
	// ExitConflict 业务冲突。
	ExitConflict = 3
	// ExitChannel 上行通道被取消或超时。
	ExitChannel = 4
	// ExitNotFound 资源不存在。
	ExitNotFound = 5
	// ExitDataIssue 数据一致性问题。
	ExitDataIssue = 6
)

// classify 把错误映射为退出码。
func classify(err error) int {
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, model.ErrSatelliteNotFound),
		errors.Is(err, model.ErrStationNotFound),
		errors.Is(err, model.ErrPassNotFound):
		return ExitNotFound
	case errors.Is(err, model.ErrBandExhausted),
		errors.Is(err, model.ErrWindowConflict),
		errors.Is(err, model.ErrSatelliteNotOperational):
		return ExitConflict
	case errors.Is(err, model.ErrUplinkTimeout),
		errors.Is(err, model.ErrUplinkUnavailable):
		return ExitChannel
	case errors.Is(err, model.ErrFrameChecksum),
		errors.Is(err, model.ErrArchiveWrite),
		errors.Is(err, model.ErrArchiveIncomplete),
		errors.Is(err, model.ErrBandOversubscribed),
		errors.Is(err, model.ErrWindowSliceAliased):
		return ExitDataIssue
	case errors.Is(err, model.ErrUnknownBand),
		errors.Is(err, model.ErrUnknownSatState),
		errors.Is(err, model.ErrUnknownStationKind),
		errors.Is(err, model.ErrInvalidSatellite),
		errors.Is(err, model.ErrInvalidStation),
		errors.Is(err, model.ErrInvalidPass),
		errors.Is(err, model.ErrInvalidGrant),
		errors.Is(err, model.ErrInvalidFrame):
		return ExitInvalidArg
	default:
		return ExitUsage
	}
}

type app struct {
	out io.Writer
	err io.Writer
}

// Run 执行一次命令行调用并返回退出码。
func Run(args []string, out, errOut io.Writer) int {
	a := &app{out: out, err: errOut}
	if len(args) < 2 {
		a.usage()
		return ExitUsage
	}
	code, err := a.route(args[1:])
	if err != nil {
		fmt.Fprintf(a.err, "错误: %v\n", err)
	}
	return code
}

func (a *app) usage() {
	fmt.Fprint(a.err, `satctl —— 低轨卫星互联网星座测控与频轨资源调度

用法:
  satctl sat list
  satctl sat show --id SAT-2401
  satctl station list
  satctl pass list [--day 2026-08-16]
  satctl pass coverage
  satctl window plan
  satctl window extend --satellite SAT-2401
  satctl spectrum allocate --workers 64 --per-worker 5 --band ka --mhz 10
  satctl telemetry ingest
  satctl archive flush
  satctl archive drill --segment-bytes 512
  satctl report scale|coverage|schedule|spectrum
  satctl serve --addr 127.0.0.1:8080
  satctl selfcheck

退出码: 0 成功 / 1 用法或未归类 / 2 参数非法 / 3 业务冲突 /
        4 上行通道取消或超时 / 5 资源不存在 / 6 数据一致性问题
`)
}

func (a *app) route(args []string) (int, error) {
	switch args[0] {
	case "sat":
		return a.runSat(args[1:])
	case "station":
		return a.runStation(args[1:])
	case "pass":
		return a.runPass(args[1:])
	case "window":
		return a.runWindow(args[1:])
	case "spectrum":
		return a.runSpectrum(args[1:])
	case "telemetry":
		return a.runTelemetry(args[1:])
	case "archive":
		return a.runArchive(args[1:])
	case "report":
		return a.runReport(args[1:])
	case "serve":
		return a.runServe(args[1:])
	case "selfcheck":
		return a.runSelfcheck()
	case "help", "-h", "--help":
		a.usage()
		return ExitOK, nil
	default:
		a.usage()
		return ExitUsage, fmt.Errorf("未知子命令 %q", args[0])
	}
}

func (a *app) emit(v any) error {
	enc := json.NewEncoder(a.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// buildRegistry 构造星座与地面站台账。
func buildRegistry() (*constellation.Registry, error) {
	reg := constellation.NewRegistry()
	if err := reg.AddSatellites(seed.Satellites()); err != nil {
		return nil, err
	}
	if err := reg.AddStations(seed.Stations()); err != nil {
		return nil, err
	}
	return reg, nil
}

// planSlots 把样例可见弧切成测控时隙。
func planSlots() []window.Slot {
	return window.SplitAll(seed.Passes(), 5*time.Minute, 120)
}

func (a *app) runSat(args []string) (int, error) {
	if len(args) == 0 {
		return ExitUsage, fmt.Errorf("sat 需要子命令 list 或 show")
	}
	reg, err := buildRegistry()
	if err != nil {
		return classify(err), err
	}
	switch args[0] {
	case "list":
		items := reg.Satellites()
		lines := make([]string, 0, len(items))
		for _, s := range items {
			lines = append(lines, constellation.DescribeSatellite(s))
		}
		if eerr := a.emit(map[string]any{
			"satellites": items,
			"describe":   lines,
			"count":      len(items),
			"trackable":  reg.TrackableSatellites(),
		}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	case "show":
		fs := flag.NewFlagSet("sat show", flag.ContinueOnError)
		fs.SetOutput(a.err)
		id := fs.String("id", "SAT-2401", "卫星编号")
		if perr := fs.Parse(args[1:]); perr != nil {
			return ExitInvalidArg, perr
		}
		s, gerr := reg.Satellite(*id)
		if gerr != nil {
			return classify(gerr), gerr
		}
		if eerr := a.emit(map[string]any{
			"satellite": s,
			"describe":  constellation.DescribeSatellite(s),
			"trackable": s.State.Trackable(),
		}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	default:
		return ExitUsage, fmt.Errorf("未知 sat 子命令 %q", args[0])
	}
}

func (a *app) runStation(args []string) (int, error) {
	if len(args) == 0 || args[0] != "list" {
		return ExitUsage, fmt.Errorf("station 需要子命令 list")
	}
	reg, err := buildRegistry()
	if err != nil {
		return classify(err), err
	}
	items := reg.Stations()
	lines := make([]string, 0, len(items))
	for _, s := range items {
		lines = append(lines, constellation.DescribeStation(s))
	}
	if eerr := a.emit(map[string]any{
		"stations": items, "describe": lines,
		"count": len(items), "antennas": reg.Antennas(),
	}); eerr != nil {
		return ExitUsage, eerr
	}
	return ExitOK, nil
}

func (a *app) runPass(args []string) (int, error) {
	if len(args) == 0 {
		return ExitUsage, fmt.Errorf("pass 需要子命令 list 或 coverage")
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("pass list", flag.ContinueOnError)
		fs.SetOutput(a.err)
		day := fs.String("day", "", "只看指定 UTC 自然日")
		if perr := fs.Parse(args[1:]); perr != nil {
			return ExitInvalidArg, perr
		}
		items := seed.Passes()
		if *day != "" {
			if _, derr := pass.ParseDay(*day); derr != nil {
				return classify(derr), derr
			}
			items = pass.OnDay(items, *day)
		} else {
			model.SortPasses(items)
		}
		lines := make([]string, 0, len(items))
		for _, p := range items {
			lines = append(lines, pass.Describe(p))
		}
		if eerr := a.emit(map[string]any{
			"day": *day, "passes": items, "describe": lines, "count": len(items),
			"seconds_total": pass.TotalSeconds(items),
		}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	case "coverage":
		return a.runPassCoverage(args[1:])
	default:
		return ExitUsage, fmt.Errorf("未知 pass 子命令 %q", args[0])
	}
}

func (a *app) runPassCoverage(args []string) (int, error) {
	fs := flag.NewFlagSet("pass coverage", flag.ContinueOnError)
	fs.SetOutput(a.err)
	if perr := fs.Parse(args); perr != nil {
		return ExitInvalidArg, perr
	}
	passes := seed.Passes()
	sum := report.Coverage(passes, seed.Days())

	// 逐段核对跨日界可见弧的分摊：它跨越的每个自然日都应当看到一个片段，
	// 且各片段时长之和等于该段可见弧的总时长。
	perCross := make([]map[string]any, 0, 2)
	splitOK := true
	incomplete := make([]string, 0, 2)
	for _, p := range passes {
		if !pass.CrossesDayBoundary(p) {
			continue
		}
		days := pass.SpanningDays(p)
		parts := make([]map[string]any, 0, len(days))
		var partSum int64
		allPresent := true
		for _, d := range days {
			clipped, ok := pass.ClipToDay(p, d)
			secs := int64(0)
			if ok {
				secs = int64(clipped.Duration() / time.Second)
			} else {
				allPresent = false
			}
			partSum += secs
			parts = append(parts, map[string]any{"day": d, "seconds": secs, "present": ok})
		}
		total := int64(p.Duration() / time.Second)
		complete := allPresent && partSum == total
		if !complete {
			splitOK = false
			incomplete = append(incomplete, p.ID)
		}
		perCross = append(perCross, map[string]any{
			"pass_id":        p.ID,
			"seconds_total":  total,
			"parts_seconds":  partSum,
			"days":           days,
			"parts":          parts,
			"split_complete": complete,
		})
	}

	ok := sum.Balanced && splitOK
	if eerr := a.emit(map[string]any{
		"summary":                 sum,
		"cross_boundary":          perCross,
		"cross_boundary_count":    len(perCross),
		"cross_boundary_split_ok": splitOK,
		"seconds_total":           sum.SecondsTotal,
		"seconds_by_day":          sum.SecondsByDay,
		"balanced":                sum.Balanced,
		"ok":                      ok,
	}); eerr != nil {
		return ExitUsage, eerr
	}
	if !ok {
		if !splitOK {
			err := fmt.Errorf("%w: 跨日界可见弧 %v 的分摊不完整，跨越的自然日中存在缺失片段",
				model.ErrInvalidPass, incomplete)
			return ExitDataIssue, err
		}
		err := fmt.Errorf("%w: 按日累计 %d 秒, 可见弧累计 %d 秒",
			model.ErrInvalidPass, sum.SecondsByDay, sum.SecondsTotal)
		return ExitDataIssue, err
	}
	return ExitOK, nil
}

func (a *app) runWindow(args []string) (int, error) {
	if len(args) == 0 {
		return ExitUsage, fmt.Errorf("window 需要子命令 plan 或 extend")
	}
	switch args[0] {
	case "plan":
		slots := planSlots()
		groups := window.Groups(slots)
		sum := report.Schedule(groups)
		if eerr := a.emit(map[string]any{
			"summary": sum, "satellites": window.SatelliteIDs(groups), "ok": sum.Consistent,
		}); eerr != nil {
			return ExitUsage, eerr
		}
		if !sum.Consistent {
			err := window.Verify(groups)
			return classify(err), err
		}
		return ExitOK, nil
	case "extend":
		return a.runWindowExtend(args[1:])
	default:
		return ExitUsage, fmt.Errorf("未知 window 子命令 %q", args[0])
	}
}

func (a *app) runWindowExtend(args []string) (int, error) {
	fs := flag.NewFlagSet("window extend", flag.ContinueOnError)
	fs.SetOutput(a.err)
	satID := fs.String("satellite", "SAT-2401", "追加时隙的卫星编号")
	if perr := fs.Parse(args); perr != nil {
		return ExitInvalidArg, perr
	}

	slots := planSlots()
	groups := window.Groups(slots)
	before := window.TotalSlots(groups)

	// 记录追加前各组的首个时隙，用于核对其他组是否被影响。
	firstBefore := make(map[string]string)
	for _, id := range window.SatelliteIDs(groups) {
		if len(groups[id]) > 0 {
			firstBefore[id] = groups[id][0].ID
		}
	}

	extra := window.Slot{
		ID:          "EXTRA-W01",
		SatelliteID: *satID,
		StationID:   "GS-KS",
		PassID:      "PASS-EXTRA",
		Start:       seed.Now(),
		End:         seed.Now().Add(4 * time.Minute),
	}
	window.Extend(groups, *satID, extra)

	changed := make([]map[string]any, 0, 2)
	for _, id := range window.SatelliteIDs(groups) {
		if id == *satID || len(groups[id]) == 0 {
			continue
		}
		if got := groups[id][0].ID; got != firstBefore[id] {
			changed = append(changed, map[string]any{
				"satellite": id, "was": firstBefore[id], "now": got,
			})
		}
	}

	sum := report.Schedule(groups)
	ok := len(changed) == 0 && sum.Consistent && sum.Slots == before+1

	if eerr := a.emit(map[string]any{
		"satellite":            *satID,
		"slots_before":         before,
		"slots_after":          sum.Slots,
		"groups":               sum.Groups,
		"misassigned":          sum.Misassigned,
		"distinct_slots":       sum.DistinctSlots,
		"consistent":           sum.Consistent,
		"other_groups_changed": changed,
		"ok":                   ok,
	}); eerr != nil {
		return ExitUsage, eerr
	}
	if !ok {
		err := window.Verify(groups)
		if err == nil {
			err = fmt.Errorf("%w: 追加时隙影响了 %d 个其他分组",
				model.ErrWindowSliceAliased, len(changed))
		}
		return classify(err), err
	}
	return ExitOK, nil
}

func (a *app) runSpectrum(args []string) (int, error) {
	if len(args) == 0 || args[0] != "allocate" {
		return ExitUsage, fmt.Errorf("spectrum 需要子命令 allocate")
	}
	fs := flag.NewFlagSet("spectrum allocate", flag.ContinueOnError)
	fs.SetOutput(a.err)
	workers := fs.Int("workers", 64, "并发申请协程数")
	perWorker := fs.Int("per-worker", 5, "每个协程的申请次数")
	bandFlag := fs.String("band", "ka", "申请频段")
	mhz := fs.Int("mhz", 10, "每次申请带宽（兆赫）")
	rounds := fs.Int("rounds", 20, "重复轮数，每轮使用全新的分配器")
	if perr := fs.Parse(args[1:]); perr != nil {
		return ExitInvalidArg, perr
	}
	if *rounds <= 0 {
		err := fmt.Errorf("%w: rounds 必须为正", model.ErrInvalidGrant)
		return classify(err), err
	}
	band, berr := model.ParseBand(*bandFlag)
	if berr != nil {
		return classify(berr), berr
	}
	if *workers <= 0 || *perWorker <= 0 || *mhz <= 0 {
		err := fmt.Errorf("%w: workers、per-worker 与 mhz 必须为正", model.ErrInvalidGrant)
		return classify(err), err
	}

	// 每轮用一个全新的分配器重复同样的并发申请，逐轮核对自洽性。
	type roundResult struct {
		Round      int  `json:"round"`
		Granted    int  `json:"granted_calls"`
		Records    int  `json:"grant_records"`
		LostGrants int  `json:"lost_grants"`
		UsedMHz    int  `json:"used_mhz"`
		WantMHz    int  `json:"expected_used_mhz"`
		Oversub    bool `json:"oversubscribed"`
		Consistent bool `json:"consistent"`
	}

	results := make([]roundResult, 0, *rounds)
	inconsistent := 0
	totalLost := 0
	var lastAlloc *spectrum.Allocator

	for r := 1; r <= *rounds; r++ {
		alloc := spectrum.NewAllocator()
		lastAlloc = alloc
		var mu sync.Mutex
		granted := 0

		var wg sync.WaitGroup
		for w := 0; w < *workers; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				for i := 0; i < *perWorker; i++ {
					if _, err := alloc.Allocate(seed.SatelliteIDFor(w), band, *mhz); err == nil {
						mu.Lock()
						granted++
						mu.Unlock()
					}
				}
			}(w)
		}
		wg.Wait()

		records := len(alloc.Grants())
		used := alloc.UsedMHz(band)
		res := roundResult{
			Round:      r,
			Granted:    granted,
			Records:    records,
			LostGrants: granted - records,
			UsedMHz:    used,
			WantMHz:    granted * *mhz,
			Oversub:    used > band.TotalMHz(),
			Consistent: records == granted && used == granted**mhz &&
				used <= band.TotalMHz() && alloc.Verify() == nil,
		}
		if !res.Consistent {
			inconsistent++
		}
		totalLost += res.LostGrants
		results = append(results, res)
	}

	ok := inconsistent == 0
	if eerr := a.emit(map[string]any{
		"band":                string(band),
		"total_mhz":           band.TotalMHz(),
		"workers":             *workers,
		"per_worker":          *perWorker,
		"rounds":              *rounds,
		"requests_per_round":  *workers * *perWorker,
		"max_grants":          band.TotalMHz() / *mhz,
		"inconsistent_rounds": inconsistent,
		"lost_grants":         totalLost,
		"rounds_detail":       results,
		"usage":               report.Spectrum(lastAlloc).Usage,
		"ok":                  ok,
	}); eerr != nil {
		return ExitUsage, eerr
	}
	if !ok {
		err := fmt.Errorf("%w: %d/%d 轮出现不一致, 累计丢失授权记录 %d 条",
			model.ErrBandOversubscribed, inconsistent, *rounds, totalLost)
		return classify(err), err
	}
	return ExitOK, nil
}

func (a *app) runTelemetry(args []string) (int, error) {
	if len(args) == 0 || args[0] != "ingest" {
		return ExitUsage, fmt.Errorf("telemetry 需要子命令 ingest")
	}
	fs := flag.NewFlagSet("telemetry ingest", flag.ContinueOnError)
	fs.SetOutput(a.err)
	clean := fs.Bool("clean", false, "只入库校验通过的帧")
	if perr := fs.Parse(args[1:]); perr != nil {
		return ExitInvalidArg, perr
	}

	frames := seed.Frames()
	if *clean {
		frames = seed.CleanFrames()
	}
	store := telemetry.NewStore()
	rep := store.IngestBatch(frames)

	payload := map[string]any{
		"submitted":      rep.Submitted,
		"ingested":       rep.Ingested,
		"not_ingested":   rep.NotIngested,
		"reported":       rep.Reported,
		"message":        rep.Message,
		"store_frames":   store.Len(),
		"corrupt_frames": seed.CorruptFrameIDs(),
		"ok":             rep.NotIngested == 0 || rep.Reported,
	}
	if eerr := a.emit(payload); eerr != nil {
		return ExitUsage, eerr
	}
	if rep.NotIngested > 0 && !rep.Reported {
		err := fmt.Errorf("%w: 提交 %d 帧, 入库 %d 帧, 但入库过程未上报任何失败",
			model.ErrArchiveIncomplete, rep.Submitted, rep.Ingested)
		return classify(err), err
	}
	if rep.Reported {
		err := fmt.Errorf("%w: %s", model.ErrFrameChecksum, rep.Message)
		return classify(err), err
	}
	return ExitOK, nil
}

func (a *app) runArchive(args []string) (int, error) {
	if len(args) == 0 {
		return ExitUsage, fmt.Errorf("archive 需要子命令 flush 或 drill")
	}
	switch args[0] {
	case "flush":
		return a.runArchiveFlush(args[1:])
	case "drill":
		return a.runArchiveDrill(args[1:])
	default:
		return ExitUsage, fmt.Errorf("未知 archive 子命令 %q", args[0])
	}
}

func (a *app) runArchiveFlush(args []string) (int, error) {
	fs := flag.NewFlagSet("archive flush", flag.ContinueOnError)
	fs.SetOutput(a.err)
	maxFrames := fs.Int("segment-frames", 4, "单段最大帧数")
	maxBytes := fs.Int("segment-bytes", 8192, "单段最大字节数")
	if perr := fs.Parse(args); perr != nil {
		return ExitInvalidArg, perr
	}

	store := telemetry.NewStore()
	if _, err := store.Ingest(seed.CleanFrames()); err != nil {
		return classify(err), err
	}
	frames := store.Frames()
	segs, ferr := archive.New(*maxFrames, *maxBytes).Flush(frames)
	if ferr != nil {
		if eerr := a.emit(map[string]any{
			"ok": false, "segments": 0, "message": ferr.Error(),
			"exit_code": classify(ferr),
		}); eerr != nil {
			return ExitUsage, eerr
		}
		return classify(ferr), ferr
	}
	verr := archive.Verify(segs, frames)
	lines := make([]string, 0, len(segs))
	for _, s := range segs {
		lines = append(lines, archive.Describe(s))
	}
	if eerr := a.emit(map[string]any{
		"segments":        len(segs),
		"describe":        lines,
		"archived_frames": archive.TotalFrames(segs),
		"archived_bytes":  archive.TotalBytes(segs),
		"ingested_frames": len(frames),
		"balanced":        verr == nil,
		"ok":              verr == nil,
	}); eerr != nil {
		return ExitUsage, eerr
	}
	if verr != nil {
		return classify(verr), verr
	}
	return ExitOK, nil
}

func (a *app) runArchiveDrill(args []string) (int, error) {
	fs := flag.NewFlagSet("archive drill", flag.ContinueOnError)
	fs.SetOutput(a.err)
	maxFrames := fs.Int("segment-frames", 4, "单段最大帧数")
	maxBytes := fs.Int("segment-bytes", 512, "单段最大字节数（故意小于最大帧长以触发段容量不变量）")
	if perr := fs.Parse(args); perr != nil {
		return ExitInvalidArg, perr
	}

	store := telemetry.NewStore()
	if _, err := store.Ingest(seed.CleanFrames()); err != nil {
		return classify(err), err
	}
	frames := store.Frames()

	oversized := 0
	for _, f := range frames {
		if f.Bytes > *maxBytes {
			oversized++
		}
	}

	segs, ferr := archive.New(*maxFrames, *maxBytes).Flush(frames)
	reported := ferr != nil
	isArchiveWrite := errors.Is(ferr, model.ErrArchiveWrite)
	msg := ""
	if ferr != nil {
		msg = ferr.Error()
	}
	ok := oversized == 0 || (reported && isArchiveWrite)

	if eerr := a.emit(map[string]any{
		"segment_bytes_limit":    *maxBytes,
		"segment_frames_limit":   *maxFrames,
		"ingested_frames":        len(frames),
		"oversized_frames":       oversized,
		"segments":               len(segs),
		"archived_frames":        archive.TotalFrames(segs),
		"flush_reported":         reported,
		"error_is_archive_write": isArchiveWrite,
		"message":                msg,
		"ok":                     ok,
	}); eerr != nil {
		return ExitUsage, eerr
	}
	if !ok {
		err := fmt.Errorf("%w: 存在 %d 帧超过单段字节上限, 但归档未上报任何失败（返回 %d 个段）",
			model.ErrArchiveIncomplete, oversized, len(segs))
		return classify(err), err
	}
	return ExitOK, nil
}

func (a *app) runReport(args []string) (int, error) {
	if len(args) == 0 {
		return ExitUsage, fmt.Errorf("report 需要子命令 scale、coverage、schedule 或 spectrum")
	}
	reg, err := buildRegistry()
	if err != nil {
		return classify(err), err
	}
	switch args[0] {
	case "scale":
		sum := report.Scale(reg)
		if eerr := a.emit(map[string]any{
			"summary": sum, "describe": report.Describe(sum),
		}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	case "coverage":
		sum := report.Coverage(seed.Passes(), seed.Days())
		if eerr := a.emit(map[string]any{"summary": sum, "ok": sum.Balanced}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	case "schedule":
		groups := window.Groups(planSlots())
		sum := report.Schedule(groups)
		if eerr := a.emit(map[string]any{"summary": sum, "ok": sum.Consistent}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	case "spectrum":
		alloc := spectrum.NewAllocator()
		for i, b := range seed.BandRequests(9) {
			if _, aerr := alloc.Allocate(seed.SatelliteIDFor(i), b, 20); aerr != nil {
				return classify(aerr), aerr
			}
		}
		sum := report.Spectrum(alloc)
		if eerr := a.emit(map[string]any{"summary": sum, "ok": sum.Consistent}); eerr != nil {
			return ExitUsage, eerr
		}
		return ExitOK, nil
	default:
		return ExitUsage, fmt.Errorf("未知 report 子命令 %q", args[0])
	}
}

func (a *app) runServe(args []string) (int, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(a.err)
	addr := fs.String("addr", "127.0.0.1:8080", "监听地址")
	if perr := fs.Parse(args); perr != nil {
		return ExitInvalidArg, perr
	}
	srv, err := httpapi.New()
	if err != nil {
		return classify(err), err
	}
	fmt.Fprintf(a.err, "satctl 正在监听 %s（接口无鉴权，仅供内网或本地演练）\n", *addr)
	if lerr := http.ListenAndServe(*addr, srv.Handler()); lerr != nil {
		return ExitUsage, lerr
	}
	return ExitOK, nil
}

func (a *app) runSelfcheck() (int, error) {
	type item struct {
		Name string `json:"name"`
		OK   bool   `json:"ok"`
		Note string `json:"note,omitempty"`
	}
	checks := make([]item, 0, 10)
	add := func(name string, ok bool, note string) {
		checks = append(checks, item{Name: name, OK: ok, Note: note})
	}
	note := func(err error) string {
		if err == nil {
			return ""
		}
		return err.Error()
	}

	reg, err := buildRegistry()
	add("星座台账装载", err == nil, note(err))

	// 跨 UTC 日界的可见弧按日裁剪后总时长守恒。
	cov := report.Coverage(seed.Passes(), seed.Days())
	add("跨日界可见弧按日裁剪守恒", cov.Balanced,
		fmt.Sprintf("按日 %d 秒 / 合计 %d 秒", cov.SecondsByDay, cov.SecondsTotal))
	add("样例含跨日界可见弧", cov.CrossBoundary == len(seed.CrossBoundaryPassIDs()), "")

	// 时隙分组互不串扰。
	groups := window.Groups(planSlots())
	beforeSlots := window.TotalSlots(groups)
	window.Extend(groups, "SAT-2401", window.Slot{
		ID: "SELFCHECK-W01", SatelliteID: "SAT-2401", StationID: "GS-KS",
		PassID: "PASS-SELFCHECK", Start: seed.Now(), End: seed.Now().Add(time.Minute),
	})
	add("时隙分组追加不串扰",
		window.Verify(groups) == nil && window.TotalSlots(groups) == beforeSlots+1,
		note(window.Verify(groups)))

	// 遥测入库失败必须上报。
	store := telemetry.NewStore()
	rep := store.IngestBatch(seed.Frames())
	add("遥测入库失败被上报", rep.NotIngested == 0 || rep.Reported, rep.Message)

	// 归档失败必须上报而不是返回空归档。
	cleanStore := telemetry.NewStore()
	if _, ierr := cleanStore.Ingest(seed.CleanFrames()); ierr != nil {
		add("遥测入库（干净帧）", false, note(ierr))
	} else {
		add("遥测入库（干净帧）", true, "")
		_, ferr := archive.New(4, 512).Flush(cleanStore.Frames())
		add("归档失败被上报", errors.Is(ferr, model.ErrArchiveWrite), note(ferr))
		segs, ferr2 := archive.New(4, 8192).Flush(cleanStore.Frames())
		add("归档正常路径与明细一致",
			ferr2 == nil && archive.Verify(segs, cleanStore.Frames()) == nil, note(ferr2))
	}

	// 频轨分配并发一致。
	alloc := spectrum.NewAllocator()
	var wg sync.WaitGroup
	var mu sync.Mutex
	granted := 0
	for w := 0; w < 32; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 4; i++ {
				if _, aerr := alloc.Allocate(seed.SatelliteIDFor(w), model.BandKa, 5); aerr == nil {
					mu.Lock()
					granted++
					mu.Unlock()
				}
			}
		}(w)
	}
	wg.Wait()
	add("频轨并发分配一致",
		alloc.Verify() == nil && len(alloc.Grants()) == granted, note(alloc.Verify()))

	if err == nil {
		add("可测控卫星判定", len(reg.TrackableSatellites()) == 3, "")
	}

	failed := 0
	for _, c := range checks {
		if !c.OK {
			failed++
		}
	}
	sort.SliceStable(checks, func(i, j int) bool { return checks[i].Name < checks[j].Name })
	if eerr := a.emit(map[string]any{
		"checks": checks, "passed": len(checks) - failed, "failed": failed, "ok": failed == 0,
	}); eerr != nil {
		return ExitUsage, eerr
	}
	if failed > 0 {
		return ExitDataIssue, fmt.Errorf("自检未通过: %d 项失败", failed)
	}
	return ExitOK, nil
}
