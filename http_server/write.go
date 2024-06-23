package http_server

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"io"
	"net/http"
	"time"
)

type WriteKeyParams struct {
	Key string `param:"key" validate:"lte=1024"`

	NotExists *string `query:"nx"`
	IfExists  *string `query:"ix"`
	Version   *int64  `query:"v"`
}

func (s *HTTPServer) WriteKey(c *CustomContext) error {
	// Have to read the body before
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.InternalError(err, "error reading the body")
	}

	var params WriteKeyParams
	if err := ValidateRequest(c, &params); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	ctx := c.Request().Context()
	logger := zerolog.Ctx(ctx)
	logger.Debug().Interface("params", params).Msg("Params")

	item, exists := tempDB[params.Key]
	if exists && params.NotExists != nil {
		return c.String(http.StatusConflict, fmt.Sprintf("Key %s already exists (nx)", params.Key))
	} else if !exists && params.IfExists != nil {
		return c.String(http.StatusConflict, fmt.Sprintf("Key %s does not exist (ix)", params.Key))
	} else if exists && params.Version != nil && item.Version != *params.Version {
		return c.String(http.StatusConflict, fmt.Sprintf("Provided version %d does not match found version %d", *params.Version, item.Version))
	} else if !exists && params.Version != nil {
		return c.String(http.StatusConflict, fmt.Sprintf("Key %s does not exist (v)", params.Key))
	}

	// Otherwise write it
	tempDB[params.Key] = Item{
		Version: time.Now().UnixNano(),
		Data:    body,
	}

	logger.Debug().Msgf("Wrote key '%s'", params.Key)

	return c.NoContent(http.StatusAccepted)
}
