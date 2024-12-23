package http_server

import (
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/danthegoodman1/httpkv/gologger"
	"github.com/danthegoodman1/httpkv/utils"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"golang.org/x/net/http2"
)

var (
	logger = gologger.NewLogger()
	tracer = otel.GetTracerProvider().Tracer("kv")
)

type HTTPServer struct {
	Echo *echo.Echo
}

type CustomValidator struct {
	validator *validator.Validate
}

var (
	db fdb.Database
)

func StartHTTPServer() *HTTPServer {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", utils.GetEnvOrDefault("HTTP_PORT", "8080")))
	if err != nil {
		logger.Error().Err(err).Msg("error creating tcp listener, exiting")
		os.Exit(1)
	}
	s := &HTTPServer{
		Echo: echo.New(),
	}
	s.Echo.HideBanner = true
	s.Echo.HidePort = true
	// s.Echo.JSONSerializer = &utils.NoEscapeJSONSerializer{}

	s.Echo.Use(CreateReqContext)
	s.Echo.Use(LoggerMiddleware)
	s.Echo.Use(middleware.CORS())
	s.Echo.Validator = &CustomValidator{validator: validator.New()}

	s.Echo.GET("/", ccHandler(s.GetOrList))
	s.Echo.GET("/:key", ccHandler(s.GetOrList))
	s.Echo.POST("/:key", ccHandler(s.WriteKey), middleware.BodyLimit("95K"))

	s.Echo.HTTPErrorHandler = customHTTPErrorHandler
	fdb.MustAPIVersion(710)
	db = fdb.MustOpenDefault()

	s.Echo.Listener = listener
	go func() {
		logger.Info().Msg("starting h2c server on " + listener.Addr().String())
		err := s.Echo.StartH2CServer("", &http2.Server{})
		// stop the broker
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Msg("failed to start h2c server, exiting")
			os.Exit(1)
		}
	}()

	return s
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

func ValidateRequest(c echo.Context, s interface{}) error {
	if err := c.Bind(s); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	// needed because POST doesn't have query param binding (https://echo.labstack.com/docs/binding#multiple-sources)
	if err := (&echo.DefaultBinder{}).BindQueryParams(c, s); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(s); err != nil {
		return err
	}
	return nil
}

func (*HTTPServer) HealthCheck(c echo.Context) error {
	return c.String(http.StatusOK, "yes")
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	err := s.Echo.Shutdown(ctx)
	return err
}

func LoggerMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		if err := next(c); err != nil {
			// default handler
			c.Error(err)
		}
		stop := time.Since(start)
		// Log otherwise
		logger := zerolog.Ctx(c.Request().Context())
		req := c.Request()
		res := c.Response()

		p := req.URL.Path
		if p == "" {
			p = "/"
		}

		cl := req.Header.Get(echo.HeaderContentLength)
		if cl == "" {
			cl = "0"
		}
		logger.Debug().Str("method", req.Method).Str("remote_ip", c.RealIP()).Str("req_uri", req.RequestURI).Str("handler_path", c.Path()).Str("path", p).Int("status", res.Status).Int64("latency_ns", int64(stop)).Str("protocol", req.Proto).Str("bytes_in", cl).Int64("bytes_out", res.Size).Msg("req recived")
		return nil
	}
}
