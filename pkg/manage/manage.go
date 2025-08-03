// Package manage provides HTTP handlers for managing photo albums.
package manage

import (
	"net/http"

	"github.com/tstromberg/livstid/pkg/livstid"
	"k8s.io/klog/v2"
)

// Server is a server for the pullsheet web app.
type Server struct {
	c    *livstid.Config
	path string
}

// New creates a new server.
func New(c *livstid.Config, path string) *Server {
	server := &Server{
		c:    c,
		path: path,
	}
	return server
}

// HideHandler hides an image or album.
func (s *Server) HideHandler() http.HandlerFunc {
	return func(_ http.ResponseWriter, _ *http.Request) {
		klog.Infof("hide")
	}
}
