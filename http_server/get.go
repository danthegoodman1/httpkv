package http_server

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/bytedance/sonic"
	"github.com/danthegoodman1/httpkv/tracing"
	"github.com/danthegoodman1/httpkv/utils"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel/attribute"
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
	ctx, span := tracing.CreateSpan(c.Request().Context(), tracer, "getItem")
	defer span.End()
	span.SetAttributes(attribute.String("params", string(utils.MustMarshal(params))))
	logger := zerolog.Ctx(ctx)
	logger.Debug().Msgf("Getting key '%s'", params.Key)

	data, err := db.ReadTransact(func(tx fdb.ReadTransaction) (interface{}, error) {
		itemBytes, err := tx.Get(fdb.Key(params.Key)).Get()
		if err != nil {
			return nil, fmt.Errorf("error in tx.Get for key %s: %w", params.Key, err)
		}

		exists := itemBytes == nil

		var item Item
		err = sonic.Unmarshal(itemBytes, &item)
		if err != nil {
			return nil, fmt.Errorf("error in sonic.Unmarshal for key %s: %w", params.Key, err)
		}

		if exists {
			c.Response().Header().Set("version", fmt.Sprint(item.Version))
			if params.Start != nil || params.End != nil {
				logger.Debug().Msgf("Using range for data %s", item.Data)
				return item.Data[utils.Deref(params.Start, 0):utils.Deref(params.End, len(item.Data)-1)], nil
			} else {
				return item.Data, nil
			}
		}

		return nil, echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Key %s not found", params.Key))
	})

	var he *echo.HTTPError
	if errors.As(err, &he) {
		return he
	}

	if err != nil {
		return fmt.Errorf("error in fdb transaction: %w", err)
	}

	return c.Blob(http.StatusOK, "application/octet-stream", data.([]byte))
}

func (s *HTTPServer) listItems(c *CustomContext, params ListParams) error {
	_, span := tracing.CreateSpan(c.Request().Context(), tracer, "listItems")
	defer span.End()
	span.SetAttributes(attribute.String("params", string(utils.MustMarshal(params))))
	var items [][]byte

	limit := utils.Deref(params.Limit, 100)

	_, err := db.ReadTransact(func(tx fdb.ReadTransaction) (interface{}, error) {
		opts := fdb.RangeOptions{
			Limit:   limit,
			Reverse: params.Reverse != nil,
		}
		keyRange := fdb.KeyRange{
			Begin: fdb.Key(utils.Deref(params.From, "")),
			End:   fdb.Key(lo.Ternary(params.Reverse != nil, utils.Deref(params.From, "\xff"), "\xff")),
		}
		iter := tx.GetRange(keyRange, opts).Iterator()
		for iter.Advance() {
			fdbItem, err := iter.Get()
			if err != nil {
				return nil, fmt.Errorf("error in fdb.Iterator.Get: %w", err)
			}

			var b []byte
			b = append(b, []byte(fdbItem.Key)...)
			if params.ListVals != nil {
				b = append(b, []byte(":")...)
				var encoded []byte
				base64.StdEncoding.Encode(encoded, fdbItem.Value)
				b = append(b, encoded...)
			}

			items = append(items, b)
		}

		return nil, nil
	})

	if err != nil {
		return fmt.Errorf("error in ReadTransaction: %w", err)
	}

	return c.Blob(http.StatusOK, "application/octet-stream", bytes.Join(items, []byte("\n")))
}
