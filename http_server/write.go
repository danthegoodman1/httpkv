package http_server

import (
	"fmt"
	"net/http"
)

func (s *HTTPServer) WriteKey(c *CustomContext) error {
	key := c.Param("key")
	return c.String(http.StatusAccepted, fmt.Sprintf("Writing key: %s", key))
}
