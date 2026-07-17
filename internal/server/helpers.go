package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxRequestBody caps request bodies read via readJSON: generous for
// the largest realistic form text (a goal/context/constraints field),
// while still bounding memory use against an abusive or buggy client.
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

// readJSON decodes r's body into dst, capped at maxRequestBody and
// rejecting unknown fields or trailing data. On error it returns a
// message safe to send back to the client (never a raw internal
// error), triaging the common decode-failure shapes into something
// specific enough to act on.
func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalTypeErr *json.UnmarshalTypeError
		var invalidUnmarshalErr *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxErr):
			return fmt.Errorf("body contains malformed JSON (at character %d)", syntaxErr.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains malformed JSON")
		case errors.As(err, &unmarshalTypeErr):
			if unmarshalTypeErr.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeErr.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeErr.Offset)
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			return fmt.Errorf("body contains unknown key %s", strings.TrimPrefix(err.Error(), "json: unknown field "))
		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxRequestBody)
		case errors.As(err, &invalidUnmarshalErr):
			panic(err) // programmer error: dst wasn't a non-nil pointer
		default:
			return err
		}
	}

	// A second Decode into an empty struct catches trailing content
	// after the first JSON value (e.g. two concatenated objects); io.EOF
	// is the only "clean" outcome.
	if err := dec.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}
