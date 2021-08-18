package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type server struct {
	mode   string
	router *gin.Engine
}

// Option configures a Deployment.
type Option func(s *server)

func WithMode(mode string) Option {
	return func(s *server) {
		s.mode = mode
	}
}

func NewServer(opts ...Option) *server {
	s := &server{mode: gin.DebugMode}

	for _, opt := range opts {
		opt(s)
	}

	gin.SetMode(s.mode)

	if s.mode != gin.TestMode {
		s.router = gin.Default()
	} else {
		s.router = gin.New()
	}

	return s
}

func (s *server) SetupRouters() *gin.Engine {
	g := s.router
	if g == nil {
		panic("router is not initialized")
	}

	g.NoRoute(func(c *gin.Context) {
		c.String(http.StatusNotFound, "Invalid path: %s", c.Request.URL.Path)
	})
	g.HandleMethodNotAllowed = true
	g.NoMethod(func(c *gin.Context) {
		c.String(http.StatusMethodNotAllowed, "Method not allowed: %s %s", c.Request.Method, c.Request.URL.Path)
	})

	g.GET("/@in/api/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	v1 := g.Group("/api/v1")
	{
		v1.GET("/health", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})
		s.InitRestRoutes(v1)
	}
	return g
}
