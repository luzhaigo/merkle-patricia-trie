package proxy

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type addRouteRequest struct {
	Hostname string `json:"hostname"`
	Backend  string `json:"backend"`
	Force    bool   `json:"force"`
}

type jsonErrorResponse struct {
	Error string `json:"error"`
}

// writeJSONError sends {"error":"..."} with Content-Type application/json.
func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(jsonErrorResponse{Error: msg})
}

func AdminHandler(rt *RouteTable) *http.ServeMux {
	sm := http.NewServeMux()

	sm.HandleFunc("POST /routes", func(w http.ResponseWriter, r *http.Request) {
		force := r.URL.Query().Get("force") == "true"
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer r.Body.Close()

		var req addRouteRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		} else if req.Hostname == "" || req.Backend == "" {
			writeJSONError(w, http.StatusBadRequest, "hostname and backend are required")
			return
		}

		if !force {
			force = req.Force
		}

		err = rt.AddRoute(req.Hostname, req.Backend, force)
		if err != nil {
			var conflict *RouteConflictError
			if errors.As(err, &conflict) {
				writeJSONError(w, http.StatusConflict, conflict.Error())
				return
			}

			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ClearProxyCache()
		route, ok := rt.Lookup(req.Hostname)
		if !ok {
			writeJSONError(w, http.StatusInternalServerError, "route missing after add")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(route)
	})

	sm.HandleFunc("GET /routes", func(w http.ResponseWriter, r *http.Request) {
		routes := rt.ListRoutes()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(routes)
	})

	sm.HandleFunc("DELETE /routes/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		err := rt.RemoveRoute(name)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ClearProxyCache()
		w.WriteHeader(http.StatusNoContent)
	})

	return sm
}
