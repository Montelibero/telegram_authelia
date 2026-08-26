package middlewares

import (
	"testing"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestLogRequestRecordsMeaningfulCompletedRequest(t *testing.T) {
	hook := logrustest.NewGlobal()
	t.Cleanup(hook.Reset)
	previousLevel := logrus.GetLevel()
	logrus.SetLevel(logrus.InfoLevel)
	t.Cleanup(func() { logrus.SetLevel(previousLevel) })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("https://auth.example.com/api/telegram/callback?code=secret&state=secret")
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)

	LogRequest(func(ctx *fasthttp.RequestCtx) {
		ctx.Redirect("/settings/admin/users", fasthttp.StatusFound)
	})(ctx)

	require.Len(t, hook.Entries, 1)
	entry := hook.LastEntry()
	require.NotNil(t, entry)
	assert.Equal(t, logrus.InfoLevel, entry.Level)
	assert.Equal(t, "HTTP request completed", entry.Message)
	assert.Equal(t, "GET", entry.Data["method"])
	assert.Equal(t, "/api/telegram/callback", entry.Data["path"])
	assert.Equal(t, fasthttp.StatusFound, entry.Data["status_code"])
	assert.Equal(t, "https://auth.example.com/settings/admin/users", entry.Data["redirect_location"])
	assert.Contains(t, entry.Data, "duration_ms")
	assert.NotContains(t, entry.Data, "query")
}

func TestLogRequestSkipsInfrastructureNoise(t *testing.T) {
	testCases := []string{
		"/api/health",
		"/api/health/ready",
		"/metrics",
		"/static/js/index.js",
		"/favicon.ico",
		"/manifest.json",
	}

	for _, path := range testCases {
		t.Run(path, func(t *testing.T) {
			hook := logrustest.NewGlobal()
			t.Cleanup(hook.Reset)
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.SetRequestURI("https://auth.example.com" + path)

			called := false
			LogRequest(func(ctx *fasthttp.RequestCtx) {
				called = true
				ctx.SetStatusCode(fasthttp.StatusOK)
			})(ctx)

			assert.True(t, called)
			assert.Empty(t, hook.Entries)
		})
	}
}
