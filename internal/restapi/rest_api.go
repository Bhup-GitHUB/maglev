package restapi

import (
	"net/http"
	"time"

	"maglev.onebusaway.org/internal/app"
)

type RestAPI struct {
	*app.Application
	rateLimiter         func(http.Handler) http.Handler
	rateLimitMiddleware *RateLimitMiddleware
}

// NewRestAPI creates a new RestAPI instance with initialized rate limiter
func NewRestAPI(app *app.Application) *RestAPI {
	middleware := NewRateLimitMiddleware(app.Config.RateLimit, time.Second)
	return &RestAPI{
		Application:         app,
		rateLimiter:         middleware.Handler(),
		rateLimitMiddleware: middleware,
	}
}

// Shutdown stops the rate limiter cleanup goroutine
func (api *RestAPI) Shutdown() {
	if api.rateLimitMiddleware != nil {
		api.rateLimitMiddleware.Stop()
	}
}
