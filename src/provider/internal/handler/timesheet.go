// Package handler 工时 API。
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
	"github.com/quanttide/qtcloud-human/src/provider/internal/store"
)

type TimesheetHandler struct {
	store *store.TimesheetStore
}

func NewTimesheetHandler(s *store.TimesheetStore) *TimesheetHandler {
	return &TimesheetHandler{store: s}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *TimesheetHandler) List(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.List())
}

func (h *TimesheetHandler) Create(w http.ResponseWriter, r *http.Request) {
	var t domain.Timesheet
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if t.UserID == "" || t.Date == "" || t.Hours <= 0 || t.Hours > 24 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "userId, date, hours(0-24) required"})
		return
	}
	created := h.store.Create(&t)
	writeJSON(w, http.StatusCreated, created)
}

func (h *TimesheetHandler) Get(w http.ResponseWriter, r *http.Request) {
	t, ok := h.store.Get(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *TimesheetHandler) Update(w http.ResponseWriter, r *http.Request) {
	var t domain.Timesheet
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	t.ID = r.PathValue("id")
	updated, ok := h.store.Update(&t)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *TimesheetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.store.Delete(r.PathValue("id")) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
