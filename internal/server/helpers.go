package server

import (
	"encoding/json"
	"net/http"
)

// maxRequestBody caps request bodies: generous for the largest
// realistic form text (a goal/context/constraints field), while still
// bounding memory use against an abusive or buggy client. Used by
// handlePreview (form bodies) and previously by the now-removed JSON
// /api/build.
const maxRequestBody = 1 << 20 // 1 MiB

// writeJSON marshals data and writes it as the response body with the
// given status code.
func writeJSON(w http.ResponseWriter, status int, data any) error {
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(js)
	return err
}
