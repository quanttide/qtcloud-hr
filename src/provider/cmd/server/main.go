// 人力资源云服务端：工时表 API。
package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/quanttide/qtcloud-human/src/provider/internal/handler"
	"github.com/quanttide/qtcloud-human/src/provider/internal/recruitment"
	"github.com/quanttide/qtcloud-human/src/provider/internal/store"
)

func main() {
	ts := store.NewTimesheetStore()
	th := handler.NewTimesheetHandler(ts)
	rs := store.DefaultRecruitmentStore()
	adapter := recruitment.NewCLIAdapter(recruitment.DefaultBinary(), recruitment.DefaultTimeout())
	audit := recruitment.NewFileAuditLogger(os.Getenv("QTCLOUD_HUMAN_ACTION_LOG_DIR"))
	rh := handler.NewRecruitmentHandler(rs, adapter, audit, handler.RecruitmentHandlerConfig{DryRunDefault: dryRunDefault()})

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
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
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
