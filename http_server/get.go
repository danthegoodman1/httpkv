package http_server

import (
	"github.com/rs/zerolog"
	"net/http"
)

func (s *HTTPServer) GetOrList(c *CustomContext) error {
	ctx := c.Request().Context()
	logger := zerolog.Ctx(ctx)
	_, isList := c.QueryParams()["list"]
	if isList {
		logger.Debug().Msg("got list request")

		return c.String(http.StatusOK, "was list")
	}

	// Otherwise we are getting a key
	if c.Path() == "/" {
		return s.HealthCheck(c)
	}

	return c.String(http.StatusOK, "was get")
}
