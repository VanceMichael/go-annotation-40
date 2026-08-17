// Package httpapi 暴露星座、可见弧与频轨占用的只读查询接口。
//
// 接口默认不带鉴权，仅面向内网或本地演练环境。若需暴露到公网，
// 必须在前置网关补充身份认证与访问控制。
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"satnet/internal/archive"
	"satnet/internal/constellation"
	"satnet/internal/model"
	"satnet/internal/pass"
	"satnet/internal/report"
	"satnet/internal/seed"
	"satnet/internal/telemetry"
	"satnet/internal/window"
)

// Server 持有星座台账。
type Server struct {
	reg *constellation.Registry
}

// New 构造 HTTP 服务。
func New() (*Server, error) {
	reg := constellation.NewRegistry()
	if err := reg.AddSatellites(seed.Satellites()); err != nil {
		return nil, err
	}
	if err := reg.AddStations(seed.Stations()); err != nil {
		return nil, err
	}
	return &Server{reg: reg}, nil
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, model.ErrSatelliteNotFound),
		errors.Is(err, model.ErrStationNotFound),
		errors.Is(err, model.ErrPassNotFound):
		return http.StatusNotFound
	case errors.Is(err, model.ErrBandExhausted),
		errors.Is(err, model.ErrWindowConflict),
		errors.Is(err, model.ErrSatelliteNotOperational):
		return http.StatusConflict
	case errors.Is(err, model.ErrFrameChecksum),
		errors.Is(err, model.ErrArchiveWrite),
		errors.Is(err, model.ErrArchiveIncomplete),
		errors.Is(err, model.ErrBandOversubscribed),
		errors.Is(err, model.ErrWindowSliceAliased):
		return http.StatusUnprocessableEntity
	case errors.Is(err, model.ErrUplinkTimeout):
		return http.StatusServiceUnavailable
	case errors.Is(err, model.ErrUplinkUnavailable):
		return http.StatusBadGateway
	case errors.Is(err, model.ErrUnknownBand),
		errors.Is(err, model.ErrInvalidPass),
		errors.Is(err, model.ErrInvalidGrant),
		errors.Is(err, model.ErrInvalidFrame):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func codeFor(err error) string {
	switch {
	case errors.Is(err, model.ErrSatelliteNotFound):
		return "satellite_not_found"
	case errors.Is(err, model.ErrStationNotFound):
		return "station_not_found"
	case errors.Is(err, model.ErrInvalidPass):
		return "invalid_pass"
	case errors.Is(err, model.ErrArchiveWrite):
		return "archive_write_failed"
	case errors.Is(err, model.ErrArchiveIncomplete):
		return "archive_incomplete"
	case errors.Is(err, model.ErrWindowSliceAliased):
		return "window_slice_aliased"
	case errors.Is(err, model.ErrBandOversubscribed):
		return "band_oversubscribed"
	default:
		return "internal_error"
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	writeJSON(w, statusFor(err), map[string]any{
		"error": map[string]string{"code": codeFor(err), "message": err.Error()},
	})
}

// Handler 返回装配好的路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/satellites", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"satellites": s.reg.Satellites(),
			"trackable":  s.reg.TrackableSatellites(),
		})
	})

	mux.HandleFunc("/api/satellites/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/satellites/"), "/")
		if id == "" {
			writeErr(w, fmt.Errorf("%w: 缺少卫星编号", model.ErrInvalidSatellite))
			return
		}
		sat, err := s.reg.Satellite(id)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"satellite": sat, "trackable": sat.State.Trackable(),
		})
	})

	mux.HandleFunc("/api/stations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"stations": s.reg.Stations(), "antennas": s.reg.Antennas(),
		})
	})

	mux.HandleFunc("/api/passes", func(w http.ResponseWriter, r *http.Request) {
		day := r.URL.Query().Get("day")
		items := seed.Passes()
		if day != "" {
			if _, err := pass.ParseDay(day); err != nil {
				writeErr(w, err)
				return
			}
			items = pass.OnDay(items, day)
		} else {
			model.SortPasses(items)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"day": day, "passes": items, "seconds_total": pass.TotalSeconds(items),
		})
	})

	mux.HandleFunc("/api/report/coverage", func(w http.ResponseWriter, r *http.Request) {
		sum := report.Coverage(seed.Passes(), seed.Days())
		writeJSON(w, http.StatusOK, map[string]any{"summary": sum, "balanced": sum.Balanced})
	})

	mux.HandleFunc("/api/report/scale", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"summary": report.Scale(s.reg)})
	})

	mux.HandleFunc("/api/report/schedule", func(w http.ResponseWriter, r *http.Request) {
		groups := window.Groups(window.SplitAll(seed.Passes(), 5*time.Minute, 120))
		writeJSON(w, http.StatusOK, map[string]any{"summary": report.Schedule(groups)})
	})

	mux.HandleFunc("/api/telemetry/ingest", func(w http.ResponseWriter, r *http.Request) {
		store := telemetry.NewStore()
		rep := store.IngestBatch(seed.Frames())
		if rep.NotIngested > 0 && !rep.Reported {
			writeErr(w, fmt.Errorf("%w: 提交 %d 帧, 入库 %d 帧, 未上报任何失败",
				model.ErrArchiveIncomplete, rep.Submitted, rep.Ingested))
			return
		}
		if rep.Reported {
			writeErr(w, fmt.Errorf("%w: %s", model.ErrFrameChecksum, rep.Message))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"report": rep})
	})

	mux.HandleFunc("/api/archive/flush", func(w http.ResponseWriter, r *http.Request) {
		store := telemetry.NewStore()
		if _, err := store.Ingest(seed.CleanFrames()); err != nil {
			writeErr(w, err)
			return
		}
		frames := store.Frames()
		segs, err := archive.New(4, 8192).Flush(frames)
		if err != nil {
			writeErr(w, err)
			return
		}
		if verr := archive.Verify(segs, frames); verr != nil {
			writeErr(w, verr)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"segments": segs, "archived_frames": archive.TotalFrames(segs),
		})
	})

	return mux
}
