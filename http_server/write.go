package http_server

import (
	"errors"
	"fmt"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/bytedance/sonic"
	"github.com/danthegoodman1/httpkv/tracing"
	"github.com/danthegoodman1/httpkv/utils"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
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
	ctx, span := tracing.CreateSpan(c.Request().Context(), tracer, "WriteKey")
	defer span.End()
	// Have to read the body before
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.InternalError(err, "error reading the body")
	}

	var params WriteKeyParams
	if err := ValidateRequest(c, &params); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	logger := zerolog.Ctx(ctx)
	logger.Debug().Interface("params", params).Msg("writing key")
	span.SetAttributes(attribute.String("params", string(utils.MustMarshal(params))))

	_, err = db.Transact(func(tx fdb.Transaction) (interface{}, error) {
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

		if exists && params.NotExists != nil {
			return nil, echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("Key %s already exists (nx)", params.Key))
		} else if !exists && params.IfExists != nil {
			return nil, echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("Key %s does not exist (ix)", params.Key))
		} else if exists && params.Version != nil && item.Version != *params.Version {
			return nil, echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("Provided version %d does not match found version %d", *params.Version, item.Version))
		} else if !exists && params.Version != nil {
			return nil, echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("Key %s does not exist (v)", params.Key))
		}

		// Otherwise write it
		itemBytes = utils.MustMarshal(Item{
			Version: time.Now().UnixNano(),
			Data:    body,
		})
		tx.Set(fdb.Key(params.Key), itemBytes)
		logger.Debug().Msgf("Wrote key '%s'", params.Key)

		// txn automatically commits
		return nil, nil
	})

	var he *echo.HTTPError
	if errors.As(err, &he) {
		return he
	}

	if err != nil {
		return fmt.Errorf("error in fdb transaction: %w", err)
	}

	// We wrote it
	return c.NoContent(http.StatusAccepted)
}
