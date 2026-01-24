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
		// cl expected to implement minimal interface. If it also exposes
		// IsDraining, include draining state per member.
		type cinfo interface {
			Members() []string
			Self() string
		}
		ci, ok := cl.(cinfo)
		if !ok {
			http.Error(w, "cluster not available", http.StatusInternalServerError)
			return
		}

		// optionally include draining flags if available
		type cdr interface {
			IsDraining(string) bool
		}
		resp := map[string]interface{}{"members": ci.Members(), "self": ci.Self()}
		if cd, ok := cl.(cdr); ok {
			drains := map[string]bool{}
			for _, m := range ci.Members() {
				drains[m] = cd.IsDraining(m)
			}
			resp["draining"] = drains
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// DrainHandler toggles draining state for a cluster node.
// Usage: POST /cluster/drain?node=<name>&drain=true|false
func DrainHandler(cl interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cl == nil {
			http.Error(w, "no cluster configured", http.StatusNotFound)
			return
		}
		type cctrl interface {
			SetDraining(string, bool)
			IsDraining(string) bool
			Members() []string
		}
		cc, ok := cl.(cctrl)
		if !ok {
			http.Error(w, "cluster does not support drain control", http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		node := r.URL.Query().Get("node")
		if node == "" {
			http.Error(w, "missing node parameter", http.StatusBadRequest)
			return
		}
		drainStr := r.URL.Query().Get("drain")
		if drainStr == "" {
			http.Error(w, "missing drain parameter (true|false)", http.StatusBadRequest)
			return
		}
		drain := false
		if drainStr == "true" || drainStr == "1" {
			drain = true
		}
		// ensure node is known
		known := false
		for _, m := range cc.Members() {
			if m == node {
				known = true
				break
			}
		}
		if !known {
			http.Error(w, "unknown node", http.StatusBadRequest)
			return
		}
		cc.SetDraining(node, drain)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"node": node, "draining": drain})
	}
}
