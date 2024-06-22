package http_server

import (
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"net/http"
)

type (
	GetOrListParams struct {
		GetParams
		ListParams
	}

	GetParams struct {
		Key string `param:"key" validate:"lte=1024"`

		Start *int `query:"start"`
		End   *int `query:"end"`
	}

	ListParams struct {
		Prefix   *string `param:"key"`
		List     *string `query:"list"`
		Reverse  *string `query:"reverse"`
		From     *string `query:"from"`
		Limit    *int    `query:"limit"`
		ListVals *string `query:"vals"`
	}
)

func (s *HTTPServer) GetOrList(c *CustomContext) error {
	var params GetOrListParams
	if err := ValidateRequest(c, &params); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	ctx := c.Request().Context()
	logger := zerolog.Ctx(ctx)
	logger.Debug().Interface("params", params).Msg("Params")
	if params.List != nil {
		logger.Debug().Msg("got list request")

		return c.String(http.StatusOK, "was list")
	}

	// Otherwise we are getting a key
	if c.Path() == "/" {
		return s.HealthCheck(c)
	}

	return c.String(http.StatusOK, "was get")
}
