// Command cuevasysimas is the server of the catalogue of places «Cuevas y simas»: the API that its clients
// (the intranet of the Club de Espeleología Katiuskas is the first) use to read and change the catalogue, the
// public routes, and the public site.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cespedes/cuevasysimas/internal/accounts"
	"github.com/cespedes/cuevasysimas/internal/areas"
	"github.com/cespedes/cuevasysimas/internal/assertion"
	"github.com/cespedes/cuevasysimas/internal/auth"
	"github.com/cespedes/cuevasysimas/internal/config"
	"github.com/cespedes/cuevasysimas/internal/httpapi"
	"github.com/cespedes/cuevasysimas/internal/osmmirror"
	"github.com/cespedes/cuevasysimas/internal/places"
	"github.com/cespedes/cuevasysimas/internal/platform/db"
	"github.com/cespedes/cuevasysimas/internal/platform/logging"
	"github.com/cespedes/cuevasysimas/internal/publicplaces"
	"github.com/cespedes/cuevasysimas/internal/publicsite"
	"github.com/cespedes/cuevasysimas/internal/telegram"
)

func main() {
	logger := logging.New()
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	// Rebuilt now that cfg is available, so that every slog call from here on is also sent to Telegram when it
	// is an error.
	telegramSender := telegram.NewSenderFromConfig(cfg.TelegramBotToken, cfg.TelegramLogChatID)
	logger = slog.New(telegram.NewLogHandler(logger.Handler(), telegramSender, slog.LevelError))
	slog.SetDefault(logger)

	// SIGTERM (systemd, docker stop) and SIGINT start an orderly shutdown (see serve) instead of killing the
	// process.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	router, err := buildRouter(cfg, pool, pool, telegramSender)
	if err != nil {
		os.Exit(1)
	}

	slog.Info("starting server", "addr", cfg.HTTPAddr)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: router}
	if err := serve(ctx, server, shutdownTimeout, func(ctx context.Context) {
		// What the requests that were still going on started in the background.
		waitWithin(ctx, httpapi.WaitBackground)
	}); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
	slog.Info("stopped")
}

// shutdownTimeout is how long an orderly shutdown may take, all of it: the requests still being answered and
// what they started in the background.
const shutdownTimeout = 25 * time.Second

// serve runs the server until it fails or ctx is cancelled (a signal). In that case it shuts it down in an
// orderly way: it stops accepting connections, lets the requests being answered finish, and then runs finish
// with what is left of timeout. It answers with the error of the server, if it failed, and nil after an
// orderly shutdown.
func serve(ctx context.Context, server *http.Server, timeout time.Duration, finish func(ctx context.Context)) error {
	failed := make(chan error, 1)
	go func() { failed <- server.ListenAndServe() }()
	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shut the server down", "error", err)
	}
	finish(shutdownCtx)
	return nil
}

// waitWithin runs wait, which has no way to be interrupted, and gives up when ctx is done.
func waitWithin(ctx context.Context, wait func()) {
	done := make(chan struct{})
	go func() {
		wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		slog.Warn("shutting down without waiting for the background work to end")
	}
}

// pinger is the one method a health check needs.
type pinger interface {
	Ping(ctx context.Context) error
}

// buildRouter wires every package's repository, service and handler together into the top-level HTTP handler
// this process serves, so that a test can build the same composition root against a real database.
func buildRouter(cfg config.Config, repoPool db.PGX, health pinger, notifier httpapi.MutationNotifier) (http.Handler, error) {
	accountsService := accounts.NewService(accounts.NewRepository(repoPool))
	// The clients prove who is acting with an assertion that they sign: the key of each is in the table of
	// issuers.
	authenticator := auth.NewAuthenticator(assertion.NewResolver(accountsService))

	placeFilesStorage, err := places.NewStorage(cfg.PlaceFilesRoot)
	if err != nil {
		slog.Error("set up place files storage", "error", err, "root", cfg.PlaceFilesRoot)
		return nil, err
	}
	placesService := places.NewService(places.NewRepository(repoPool), placeFilesStorage, accountsService)
	areasService := areas.NewService(areas.NewRepository(repoPool), accountsService)
	// No address, or "off", turns the lookup of the zone of a point off (the tests, an offline host).
	if cfg.NominatimURL != "" && cfg.NominatimURL != "off" {
		nominatim := areas.NewNominatim(cfg.NominatimURL) // One client, so its requests are spaced together.
		areasService.WithBoundaries(nominatim).WithGeocoder(nominatim)
	}

	publicSite, err := publicsite.NewHandler(placesService, cfg.PublicSiteURL, publicsite.Legal{
		Name: cfg.PublicSiteLegalName, Address: cfg.PublicSiteLegalAddress, Email: cfg.PublicSiteLegalEmail,
	}, cfg.PublicRateLimit, cfg.PublicRateWindow)
	if err != nil {
		slog.Error("set up the public site", "error", err, "url", cfg.PublicSiteURL)
		return nil, err
	}

	return httpapi.NewRouter(httpapi.Routes{
		Health:       httpapi.NewHealthHandler(health),
		Places:       places.NewHandler(placesService, authenticator, notifier),
		Areas:        areas.NewHandler(areasService, authenticator, notifier),
		OSM:          osmmirror.NewHandler(osmmirror.NewService(osmmirror.NewRepository(repoPool)), authenticator),
		PublicPlaces: publicplaces.NewHandler(placesService, authenticator, cfg.PublicRateLimit, cfg.PublicRateWindow),
		PublicSite:   publicSite,
	}), nil
}
