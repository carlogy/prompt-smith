// promptsmith - generate portable, skill-aware prompts for any LLM or agent harness.
// Copyright (C) 2026 carlogy
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
