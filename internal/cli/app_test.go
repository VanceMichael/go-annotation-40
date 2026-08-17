package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"satnet/internal/cli"
)

func run(t *testing.T, args ...string) (int, map[string]any, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := cli.Run(append([]string{"satctl"}, args...), &out, &errOut)
	payload := map[string]any{}
	if s := strings.TrimSpace(out.String()); s != "" {
		if err := json.Unmarshal([]byte(s), &payload); err != nil {
			t.Fatalf("输出不是合法 JSON: %v\n%s", err, s)
		}
	}
	return code, payload, errOut.String()
}

func TestCLISelfcheckPasses(t *testing.T) {
	code, payload, stderr := run(t, "selfcheck")
	if code != cli.ExitOK {
		t.Fatalf("自检退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["ok"] != true {
		t.Fatalf("自检应全部通过: %v", payload)
	}
}

func TestCLIPassCoverageBalanced(t *testing.T) {
	code, payload, stderr := run(t, "pass", "coverage")
	if code != cli.ExitOK {
		t.Fatalf("覆盖统计退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["balanced"] != true {
		t.Fatalf("按日累计应等于可见弧累计: by_day=%v total=%v",
			payload["seconds_by_day"], payload["seconds_total"])
	}
	if payload["cross_boundary_split_ok"] != true {
		t.Fatalf("跨日界可见弧应被完整分摊到相邻两日: %v", payload["cross_boundary"])
	}
	if payload["cross_boundary_count"] == float64(0) {
		t.Fatalf("样例数据应包含跨日界可见弧: %v", payload)
	}
}

func TestCLIPassListByDay(t *testing.T) {
	code, payload, stderr := run(t, "pass", "list", "--day", "2026-08-17")
	if code != cli.ExitOK {
		t.Fatalf("按日列出退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["count"] == float64(0) {
		t.Fatalf("次日应当有可见弧片段: %v", payload)
	}
}

func TestCLIWindowExtendKeepsGroupsIntact(t *testing.T) {
	code, payload, stderr := run(t, "window", "extend", "--satellite", "SAT-2401")
	if code != cli.ExitOK {
		t.Fatalf("追加时隙退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["misassigned"] != float64(0) {
		t.Fatalf("不应有归属错误的时隙: %v", payload["misassigned"])
	}
	if payload["consistent"] != true {
		t.Fatalf("分组应自洽: %v", payload)
	}
	changed, _ := payload["other_groups_changed"].([]any)
	if len(changed) != 0 {
		t.Fatalf("其他分组不应被影响: %v", changed)
	}
}

func TestCLIWindowPlanConsistent(t *testing.T) {
	code, payload, stderr := run(t, "window", "plan")
	if code != cli.ExitOK {
		t.Fatalf("排布退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["ok"] != true {
		t.Fatalf("分组应自洽: %v", payload)
	}
}

func TestCLISpectrumAllocateConsistent(t *testing.T) {
	code, payload, stderr := run(t, "spectrum", "allocate",
		"--workers", "32", "--per-worker", "4", "--band", "qv", "--mhz", "5",
		"--rounds", "10")
	if code != cli.ExitOK {
		t.Fatalf("并发分配退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["inconsistent_rounds"] != float64(0) {
		t.Fatalf("不应有不一致的轮次: %v（累计丢失授权 %v 条）",
			payload["inconsistent_rounds"], payload["lost_grants"])
	}
	if payload["lost_grants"] != float64(0) {
		t.Fatalf("不应丢失授权记录: %v", payload["lost_grants"])
	}
}

func TestCLITelemetryIngestReportsFailure(t *testing.T) {
	code, payload, _ := run(t, "telemetry", "ingest")
	if code != cli.ExitDataIssue {
		t.Fatalf("含损坏帧时退出码应为 %d, 实际 %d\n%v", cli.ExitDataIssue, code, payload)
	}
	if payload["reported"] != true {
		t.Fatalf("入库失败必须被上报: %v", payload)
	}
}

func TestCLITelemetryIngestCleanSucceeds(t *testing.T) {
	code, payload, stderr := run(t, "telemetry", "ingest", "--clean")
	if code != cli.ExitOK {
		t.Fatalf("干净批次退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["not_ingested"] != float64(0) {
		t.Fatalf("干净批次应全部入库: %v", payload)
	}
}

func TestCLIArchiveDrillReportsFailure(t *testing.T) {
	code, payload, stderr := run(t, "archive", "drill", "--segment-bytes", "512")
	if code != cli.ExitOK {
		t.Fatalf("归档演练退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["flush_reported"] != true {
		t.Fatalf("归档失败必须被上报: %v", payload)
	}
	if payload["error_is_archive_write"] != true {
		t.Fatalf("上报的错误应可判定为归档写出失败: %v", payload)
	}
}

func TestCLIArchiveFlushSucceeds(t *testing.T) {
	code, payload, stderr := run(t, "archive", "flush")
	if code != cli.ExitOK {
		t.Fatalf("归档退出码应为 0, 实际 %d\n%s", code, stderr)
	}
	if payload["balanced"] != true {
		t.Fatalf("归档应与入库明细一致: %v", payload)
	}
}

func TestCLISatShowAndNotFound(t *testing.T) {
	code, payload, _ := run(t, "sat", "show", "--id", "SAT-2403")
	if code != cli.ExitOK {
		t.Fatalf("查询卫星退出码应为 0, 实际 %d", code)
	}
	if payload["trackable"] != true {
		t.Fatalf("安全模式卫星应可测控: %v", payload)
	}
	code, _, _ = run(t, "sat", "show", "--id", "SAT-NOPE")
	if code != cli.ExitNotFound {
		t.Fatalf("不存在的卫星退出码应为 %d, 实际 %d", cli.ExitNotFound, code)
	}
}

func TestCLIReportsSucceed(t *testing.T) {
	for _, sub := range []string{"scale", "coverage", "schedule", "spectrum"} {
		code, payload, stderr := run(t, "report", sub)
		if code != cli.ExitOK {
			t.Fatalf("report %s 退出码应为 0, 实际 %d\n%s", sub, code, stderr)
		}
		if len(payload) == 0 {
			t.Fatalf("report %s 应有输出", sub)
		}
	}
}

func TestCLIInvalidBandIsInvalidArg(t *testing.T) {
	code, _, _ := run(t, "spectrum", "allocate", "--band", "zz")
	if code != cli.ExitInvalidArg {
		t.Fatalf("未知频段退出码应为 %d, 实际 %d", cli.ExitInvalidArg, code)
	}
}
