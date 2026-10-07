package httpapi

import (
	"net/http"

	"example.com/apiary-colony-service/internal/domain"
)

func (s *Server) handleApiaryEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.deps.Audit.ListByApiary(r.PathValue("apiaryId"))
	if err != nil {
		writeError(w, err)
		return
	}
	if events == nil {
		events = []domain.Event{}
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleHiveEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.deps.Audit.ListByHive(r.PathValue("hiveId"))
	if err != nil {
		writeError(w, err)
		return
	}
	if events == nil {
		events = []domain.Event{}
	}
	writeJSON(w, http.StatusOK, events)
}
