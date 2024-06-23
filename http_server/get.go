package http_server

import (
	"bytes"
	"fmt"
	"github.com/danthegoodman1/GoAPITemplate/utils"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
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

		return s.listItems(c, params.ListParams)
	}

	// Otherwise we are getting a key
	if c.Path() == "/" {
		return s.HealthCheck(c)
	}

	return s.getItem(c, params.GetParams)
}

func (s *HTTPServer) getItem(c *CustomContext, params GetParams) error {
	ctx := c.Request().Context()
	logger := zerolog.Ctx(ctx)
	logger.Debug().Msgf("Getting key '%s'", params.Key)

	item, exists := tempDB[params.Key]
	if exists {
		fmt.Println("Got version", item.Version, string(item.Data))
		c.Response().Header().Set("version", fmt.Sprint(item.Version))
		var data []byte
		if params.Start != nil || params.End != nil {
			logger.Debug().Msgf("Using range for data %s", item.Data)
			data = item.Data[utils.Deref(params.Start, 0):utils.Deref(params.End, len(item.Data)-1)]
		} else {
			data = item.Data
		}
		return c.Blob(http.StatusOK, "application/octet-stream", data)
	}

	return c.String(http.StatusNotFound, fmt.Sprintf("Key %s not found", params.Key))
}

func (s *HTTPServer) listItems(c *CustomContext, params ListParams) error {
	var items [][]byte

	// TODO: Ignoring offset and reverse
	limit := utils.Deref(params.Limit, 100)
	sep := lo.Ternary(params.ListVals == nil, "\n", "\n\n")

	i := 0
	for key, val := range tempDB {
		var b []byte
		b = append(b, []byte(key)...)
		if params.ListVals != nil {
			b = append(b, []byte("\n")...)
			b = append(b, val.Data...)
		}

		items = append(items, b)

		if i >= limit {
			fmt.Println("breaking at limit", limit)
			break
		}
		i++
	}

	return c.Blob(http.StatusOK, "application/octet-stream", bytes.Join(items, []byte(sep)))
}
