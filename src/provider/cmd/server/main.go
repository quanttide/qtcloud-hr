// 人力资源云服务端：工时表 API。
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/quanttide/qtcloud-human/src/provider/internal/handler"
	"github.com/quanttide/qtcloud-human/src/provider/internal/recruitment"
	"github.com/quanttide/qtcloud-human/src/provider/internal/store"
)

func main() {
	if err := configureRecruitmentCacheHome(); err != nil {
		log.Fatalf("configure recruitment cache: %v", err)
	}

	ts := store.NewTimesheetStore()
	th := handler.NewTimesheetHandler(ts)
	rs := store.DefaultRecruitmentStore()
	adapter := recruitment.NewCLIAdapter(recruitment.DefaultBinary(), recruitment.DefaultTimeout())
	adapter.ArgsPrefix = recruitment.DefaultArgsPrefix()
	audit := recruitment.NewFileAuditLogger(os.Getenv("QTCLOUD_HUMAN_ACTION_LOG_DIR"))
	rh := handler.NewRecruitmentHandler(rs, adapter, audit, handler.RecruitmentHandlerConfig{
		DryRunDefault:    dryRunDefault(),
		AllowRealActions: allowRealRecruitmentActions(),
		ResumeCacheRoot:  recruitmentResumeCacheRoot(),
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /timesheets", th.List)
	mux.HandleFunc("POST /timesheets", th.Create)
	mux.HandleFunc("GET /timesheets/{id}", th.Get)
	mux.HandleFunc("PUT /timesheets/{id}", th.Update)
	mux.HandleFunc("DELETE /timesheets/{id}", th.Delete)
	rh.RegisterRoutes(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := ":8080"
	if a := os.Getenv("LISTEN_ADDR"); a != "" {
		addr = a
	}
	log.Printf("qtcloud-human starting on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(mux, corsOrigins())); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func configureRecruitmentCacheHome() error {
	if strings.TrimSpace(os.Getenv("XDG_CACHE_HOME")) != "" {
		return nil
	}
	if override := strings.TrimSpace(os.Getenv("QTCLOUD_HUMAN_CACHE_HOME")); override != "" {
		return os.Setenv("XDG_CACHE_HOME", override)
	}
	return os.Setenv("XDG_CACHE_HOME", filepath.Join(os.TempDir(), "qtcloud-human", "cache"))
}

func recruitmentResumeCacheRoot() string {
	cacheHome := strings.TrimSpace(os.Getenv("XDG_CACHE_HOME"))
	if cacheHome == "" {
		return ""
	}
	return filepath.Join(cacheHome, "qtrecurit", "inbox", "resume-files")
}

func dryRunDefault() bool {
	value := os.Getenv("QTRECURIT_DRY_RUN_DEFAULT")
	if value == "" {
		return true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return true
	}
	return parsed
}

func allowRealRecruitmentActions() bool {
	value := os.Getenv("QTCLOUD_HUMAN_ALLOW_REAL_RECRUITMENT_ACTIONS")
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func corsOrigins() []string {
	value := os.Getenv("QTCLOUD_HUMAN_CORS_ORIGINS")
	if value == "" {
		return []string{
			"http://127.0.0.1:5080",
			"http://localhost:5080",
			"http://127.0.0.1:5081",
			"http://localhost:5081",
			"http://127.0.0.1:5082",
			"http://localhost:5082",
			"http://127.0.0.1:5083",
			"http://localhost:5083",
		}
	}
	origins := []string{}
	for _, origin := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func withCORS(next http.Handler, allowedOrigins []string) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Operator, X-Recruitment-Permission")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
