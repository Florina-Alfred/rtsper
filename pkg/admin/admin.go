package admin

import (
	"encoding/json"
	"net/http"

	"redalf.de/rtsper/pkg/topic"
)

// StatusHandler returns an HTTP handler that serves manager status
func StatusHandler(mgr *topic.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st := mgr.Status()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(st)
	}
}
