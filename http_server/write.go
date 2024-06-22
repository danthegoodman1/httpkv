package http_server

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"net/http"
)

type WriteKeyParams struct {
	Key string `param:"key" validate:"lte=1024"`

	NotExists *string `query:"nx"`
	IfExists  *string `query:"ix"`
}

func (s *HTTPServer) WriteKey(c *CustomContext) error {
	var params WriteKeyParams
	if err := ValidateRequest(c, &params); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	ctx := c.Request().Context()
	logger := zerolog.Ctx(ctx)
	logger.Debug().Interface("params", params).Msg("Params")

	return c.String(http.StatusAccepted, fmt.Sprintf("Writing key: %s", params.Key))
}
