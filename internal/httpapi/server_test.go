package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"satnet/internal/httpapi"
)

func newServer(t *testing.T) http.Handler {
	t.Helper()
	s, err := httpapi.New()
	if err != nil {
		t.Fatalf("构造服务失败: %v", err)
	}
	return s.Handler()
}

func get(t *testing.T, h http.Handler, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	payload := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("响应不是合法 JSON: %v\n%s", err, rec.Body.String())
		}
	}
	return rec.Code, payload
}

func TestHealthz(t *testing.T) {
	code, payload := get(t, newServer(t), "/healthz")
	if code != http.StatusOK || payload["ok"] != true {
		t.Fatalf("健康检查异常: %d %v", code, payload)
	}
}

func TestListEndpoints(t *testing.T) {
	h := newServer(t)
	for _, p := range []string{"/api/satellites", "/api/stations", "/api/passes"} {
		if code, payload := get(t, h, p); code != http.StatusOK || len(payload) == 0 {
			t.Fatalf("%s 异常: %d %v", p, code, payload)
		}
	}
}

func TestSatelliteNotFound(t *testing.T) {
	code, _ := get(t, newServer(t), "/api/satellites/SAT-NOPE")
	if code != http.StatusNotFound {
		t.Fatalf("不存在的卫星应返回 404, 实际 %d", code)
	}
}

func TestPassesByDayIncludesNextDay(t *testing.T) {
	code, payload := get(t, newServer(t), "/api/passes?day=2026-08-17")
	if code != http.StatusOK {
		t.Fatalf("按日查询应返回 200, 实际 %d", code)
	}
	items, ok := payload["passes"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("次日应当有可见弧片段: %v", payload)
	}
}

func TestPassesBadDay(t *testing.T) {
	code, _ := get(t, newServer(t), "/api/passes?day=2026-8-16-x")
	if code != http.StatusBadRequest {
		t.Fatalf("非法自然日应返回 400, 实际 %d", code)
	}
}

func TestCoverageBalanced(t *testing.T) {
	code, payload := get(t, newServer(t), "/api/report/coverage")
	if code != http.StatusOK {
		t.Fatalf("覆盖报表应返回 200, 实际 %d", code)
	}
	if payload["balanced"] != true {
		t.Fatalf("按日裁剪应守恒: %v", payload)
	}
}

func TestScheduleAndScale(t *testing.T) {
	h := newServer(t)
	if code, payload := get(t, h, "/api/report/scale"); code != http.StatusOK || payload["summary"] == nil {
		t.Fatalf("规模报表异常: %d %v", code, payload)
	}
	if code, payload := get(t, h, "/api/report/schedule"); code != http.StatusOK || payload["summary"] == nil {
		t.Fatalf("时隙报表异常: %d %v", code, payload)
	}
}

func TestTelemetryIngestReportsFailure(t *testing.T) {
	code, payload := get(t, newServer(t), "/api/telemetry/ingest")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("含损坏帧时应返回 422, 实际 %d %v", code, payload)
	}
}

func TestArchiveFlushSucceeds(t *testing.T) {
	code, payload := get(t, newServer(t), "/api/archive/flush")
	if code != http.StatusOK || payload["segments"] == nil {
		t.Fatalf("归档接口异常: %d %v", code, payload)
	}
}
