package handlers

import (
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/storage"
)

func adminAPIParse(ctx *middlewares.AutheliaCtx, destination any) bool {
	if err := ctx.ParseBody(destination); err != nil {
		ctx.ReplyBadRequest()
		return false
	}
	return true
}

func adminAPIActor(ctx *middlewares.AutheliaCtx) string {
	userSession, err := ctx.GetSession()
	if err != nil {
		return ""
	}
	return userSession.Username
}

func adminAPIRespond(ctx *middlewares.AutheliaCtx, data any, status int, err error) {
	if err != nil {
		adminAPIError(ctx, err)
		return
	}
	if err = ctx.ReplyJSON(middlewares.OKResponse{Status: "OK", Data: data}, status); err != nil {
		adminAPIError(ctx, err)
	}
}

func adminAPIError(ctx *middlewares.AutheliaCtx, err error) {
	status := fasthttp.StatusInternalServerError
	switch {
	case errors.Is(err, storage.ErrMTLUserNotFound), errors.Is(err, storage.ErrMTLGroupNotFound), errors.Is(err, storage.ErrMTLIdentityNotFound), errors.Is(err, storage.ErrMTLMembershipNotFound), errors.Is(err, storage.ErrMTLRegistrationNotFound):
		status = fasthttp.StatusNotFound
	case errors.Is(err, storage.ErrMTLVersionConflict), errors.Is(err, storage.ErrMTLConflict), errors.Is(err, storage.ErrMTLPrimaryEmailRequired), errors.Is(err, storage.ErrMTLRegistrationTerminal):
		status = fasthttp.StatusConflict
	case errors.Is(err, storage.ErrMTLRegistrationIncomplete):
		status = fasthttp.StatusBadRequest
	}
	_ = ctx.ReplyJSON(middlewares.ErrorResponse{Status: "KO", Message: fasthttp.StatusMessage(status)}, status)
}
