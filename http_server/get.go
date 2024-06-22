package http_server

import (
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"net/http"
)

type GetOrListParams struct {
	Key string `param:"key" validate:"lte=1024"`

	List *string `query:"list"`
}

func (s *HTTPServer) GetOrList(c *CustomContext) error {
	var params GetOrListParams
	if err := ValidateRequest(c, &params); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	ctx := c.Request().Context()
	logger := zerolog.Ctx(ctx)
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
