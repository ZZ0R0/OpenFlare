package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"
)

// Recovery is a middleware that recovers from panics and returns 500.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				reqID := GetRequestID(r.Context())
				stack := string(debug.Stack())

				entry := map[string]interface{}{
					"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
					"level":      "error",
					"msg":        "panic recovered",
					"request_id": reqID,
					"panic":      fmt.Sprintf("%v", rec),
					"stack":      stack,
				}
				data, _ := json.Marshal(entry)
				logger.Println(string(data))

				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
