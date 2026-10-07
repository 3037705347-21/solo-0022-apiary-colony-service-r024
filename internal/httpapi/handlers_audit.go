package httpapi

import (
	"net/http"

	"example.com/apiary-colony-service/internal/domain"
)

func (s *Server) handleApiaryEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.deps.Audit.ByApiary(r.PathValue("apiaryId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ensureEvents(events))
}

func (s *Server) handleHiveEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.deps.Audit.ByHive(r.PathValue("hiveId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ensureEvents(events))
}

func ensureEvents(events []domain.AuditEvent) []domain.AuditEvent {
	if events == nil {
		return []domain.AuditEvent{}
	}
	return events
}
