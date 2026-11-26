package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

type logFieldsKey struct{}

// requestLogFields is a per-request mutable box for extra fields the final "request" log line should
// include, filled in by middleware that runs after RequestLogger but knows something it does not: who is
// making the call (the authentication). It works by reference: RequestLogger puts a pointer to this box in
// the context before dispatching, so a value set deep in the chain is still visible once the chain returns.
type requestLogFields struct {
	Subject string
}

// SetRequestSubject records who makes the request (the issuer and the subject, never a name) for the log
// line RequestLogger writes once the request finishes. A no-op outside a request wrapped by RequestLogger.
func SetRequestSubject(ctx context.Context, subject string) {
	if fields, ok := ctx.Value(logFieldsKey{}).(*requestLogFields); ok {
		fields.Subject = subject
	}
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		fields := &requestLogFields{}
		r = r.WithContext(context.WithValue(r.Context(), logFieldsKey{}, fields))

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		var attrs []any
		if fields.Subject != "" {
			attrs = append(attrs, "subject", fields.Subject)
		}
		slog.Info(fmt.Sprintf("%s %s %d %dms", r.Method, r.URL.Path, rec.status,
			time.Since(start).Milliseconds()), attrs...)
	})
}

// Recoverer turns a panic in a handler into a 500 response instead of
// crashing the server.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "error", rec, "stack", string(debug.Stack()))
				WriteError(w, http.StatusInternalServerError, "internal_error", "unexpected server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
