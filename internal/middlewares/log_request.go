package middlewares

import (
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/logging"
)

// LogRequest records completed user-facing HTTP activity at info level.
func LogRequest(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if !shouldLogRequest(string(ctx.Path())) {
			next(ctx)
			return
		}

		started := time.Now()
		next(ctx)

		fields := logrus.Fields{
			logging.FieldMethod:     string(ctx.Method()),
			logging.FieldPath:       string(ctx.Path()),
			logging.FieldRemoteIP:   RequestCtxRemoteIP(ctx).String(),
			logging.FieldStatusCode: ctx.Response.StatusCode(),
			"duration_ms":           time.Since(started).Milliseconds(),
		}
		if location := ctx.Response.Header.Peek(fasthttp.HeaderLocation); len(location) != 0 {
			fields["redirect_location"] = string(location)
		}

		logging.Logger().WithFields(fields).Info("HTTP request completed")
	}
}

func shouldLogRequest(path string) bool {
	switch path {
	case "/api/health", "/metrics", "/favicon.ico", "/manifest.json", "/robots.txt":
		return false
	default:
		return !strings.HasPrefix(path, "/api/health/") && !strings.HasPrefix(path, "/static/")
	}
}
