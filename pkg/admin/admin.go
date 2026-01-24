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

// ClusterHandler provides basic cluster info if a cluster manager is available.
func ClusterHandler(cl interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cl == nil {
			http.Error(w, "no cluster configured", http.StatusNotFound)
			return
		}
		// cl expected to implement minimal interface
		type cinfo interface {
			Members() []string
			Self() string
		}
		ci, ok := cl.(cinfo)
		if !ok {
			http.Error(w, "cluster not available", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"members": ci.Members(), "self": ci.Self()})
	}
}
