package httpapi

import "net/http"

// ResourceHandler lets each domain package own and register its own routes, including whatever
// authentication each one needs.
type ResourceHandler interface {
	RegisterRoutes(mux *http.ServeMux)
}

// Routes are the groups of routes of the server: the one for its clients (the API that people the clients vouch
// for use to read and change the catalogue), the public ones, and the pages of the public site.
type Routes struct {
	Health       ResourceHandler
	Places       ResourceHandler
	Areas        ResourceHandler
	OSM          ResourceHandler
	PublicPlaces ResourceHandler
	PublicSite   ResourceHandler
}

func NewRouter(routes Routes) http.Handler {
	mux := http.NewServeMux()

	routes.Health.RegisterRoutes(mux)
	routes.Places.RegisterRoutes(mux)
	routes.Areas.RegisterRoutes(mux)
	routes.OSM.RegisterRoutes(mux)
	routes.PublicPlaces.RegisterRoutes(mux)
	routes.PublicSite.RegisterRoutes(mux)

	// Recoverer has to be the inner layer, closer to mux, not the outer one: RequestLogger's own
	// next.ServeHTTP call has no recover of its own, so a panic reaching it from further in would unwind
	// straight past the code that logs the line. With Recoverer innermost it turns the panic into an
	// ordinary 500 response before control returns to RequestLogger, which then logs its line as for any
	// other response.
	return RequestLogger(Recoverer(mux))
}
