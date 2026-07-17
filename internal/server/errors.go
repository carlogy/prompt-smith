package server

import "net/http"

// errorResponse is the JSON shape for every error this server reports,
// so a client can always safely parse {"error": "..."} regardless of
// which endpoint or status code produced it.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSONError writes message as a JSON error body. It's best-effort:
// if even this write fails (a broken connection, most likely), there's
// nothing further to do about it.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	_ = writeJSON(w, status, errorResponse{Error: message})
}

// serverError logs err with request context and reports a generic 500
// to the client - the specifics stay in the log, never in the response.
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
	writeJSONError(w, http.StatusInternalServerError, "the server encountered a problem and could not process the request")
}
