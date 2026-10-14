// Package server wires all components together and runs the HTTP server.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/admin"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/api"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/billing"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/bus"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/catalog"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/config"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/db"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/gateway"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/httpx"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/keyauth"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/payments"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/payments/paddle"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/payments/sslcommerz"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/ratelimit"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/secret"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/session"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/upstream"
)

type App struct {
	Cfg     *config.Config
	Log     *slog.Logger
	Pool    *pgxpool.Pool
	Store   *store.Store
	Catalog *catalog.Catalog
	Ledger  *billing.Ledger
	Bus     *bus.Bus
	Limiter *ratelimit.Limiter
	Handler http.Handler
}

// New connects to the database, applies migrations and builds the handler tree.
func New(ctx context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
	pool, err := db.Connect(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	box, err := secret.NewBox(cfg.EncryptionKey)
	if err != nil {
		pool.Close()
		return nil, err
	}
	st := store.New(pool)
	if err := bootstrap(ctx, cfg, st, box, log); err != nil {
		pool.Close()
		return nil, err
	}

	client := upstream.NewClient(cfg.UpstreamTimeout, cfg.AppName, cfg.WebURL)
	cat := catalog.New(st, box, client, log)
	if err := cat.Reload(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	ledger := billing.NewLedger(st, log, cfg.FlushInterval)
	keys := keyauth.New(st)
	limiter := ratelimit.New()
	sess := session.New(cfg.JWTSecret, cfg.InternalSecret, st)
	b := bus.New(pool, log)
	b.On(bus.KindKey, keys.InvalidateHash)
	b.On(bus.KindUser, func(id string) {
		keys.InvalidateUser(id)
		ledger.Invalidate(id)
		sess.Invalidate(id)
	})
	b.On(bus.KindCatalog, func(string) {
		rctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := cat.Reload(rctx); err != nil {
			log.Warn("catalog reload (bus) failed", "err", err)
		}
	})

	mux := http.NewServeMux()
	(&gateway.Handler{
		Keys: keys, Catalog: cat, Ledger: ledger, Upstream: client, Limiter: limiter, Log: log,
		MaxBody: cfg.MaxBodyBytes, DefaultAnthropicMaxTokens: 8192,
	}).Register(mux)
	(&api.Server{St: st, Sess: sess, Ledger: ledger, Catalog: cat, Payments: buildPayments(cfg, log), Bus: b, Cfg: cfg, Log: log}).Register(mux)
	(&admin.Server{St: st, Sess: sess, Catalog: cat, Ledger: ledger, Bus: b, Box: box, Log: log}).Register(mux)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		pctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(pctx); err != nil {
			httpx.JSON(w, 503, map[string]string{"status": "database unavailable"})
			return
		}
		httpx.JSON(w, 200, map[string]any{"status": "ready", "models": len(cat.Snapshot().Models), "billing_queue": ledger.QueueLen()})
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, 200, map[string]string{"name": cfg.AppName + " API", "docs": cfg.WebURL + "/docs"})
	})

	handler := httpx.Wrap(httpx.CORS(mux, cfg.CORSOrigins), log)
	return &App{Cfg: cfg, Log: log, Pool: pool, Store: st, Catalog: cat, Ledger: ledger, Bus: b, Limiter: limiter, Handler: handler}, nil
}

// buildPayments registers the available top-up methods.
//
// To add a Bangladeshi method (bKash, Nagad, ...): implement payments.Provider
// in internal/payments/<name>, then register it here behind its env config.
func buildPayments(cfg *config.Config, log *slog.Logger) *payments.Registry {
	reg := payments.NewRegistry()
	if cfg.Paddle.APIKey != "" && cfg.Paddle.WebhookSecret != "" {
		reg.Register(paddle.New(cfg.Paddle.APIKey, cfg.Paddle.WebhookSecret, cfg.Paddle.Environment, cfg.Paddle.ProductID))
	} else {
		log.Warn("Paddle is not configured; international top-ups are disabled")
		reg.AddUpcoming(payments.Method{ID: "paddle", Name: "Card or PayPal", Description: "International cards and PayPal.",
			Currency: "USD", Regions: []string{payments.RegionGlobal}, Logos: []string{"visa", "mastercard", "paypal"}})
	}
	if cfg.SSLCommerz.StoreID != "" && cfg.SSLCommerz.StorePasswd != "" {
		reg.Register(sslcommerz.New(cfg.SSLCommerz.StoreID, cfg.SSLCommerz.StorePasswd, cfg.SSLCommerz.Sandbox))
	} else {
		reg.AddUpcoming(payments.Method{ID: "bkash", Name: "bKash", Description: "Pay in BDT with bKash.",
			Currency: "BDT", Regions: []string{payments.RegionBD}, Logos: []string{"bkash"}})
		reg.AddUpcoming(payments.Method{ID: "nagad", Name: "Nagad", Description: "Pay in BDT with Nagad.",
			Currency: "BDT", Regions: []string{payments.RegionBD}, Logos: []string{"nagad"}})
	}
	return reg
}

// bootstrap creates the default upstream provider and promotes admin emails.
func bootstrap(ctx context.Context, cfg *config.Config, st *store.Store, box *secret.Box, log *slog.Logger) error {
	providers, err := st.ListProviders(ctx)
	if err != nil {
		return err
	}
	if len(providers) == 0 && cfg.OpenRouterKey != "" {
		enc, err := box.Encrypt(cfg.OpenRouterKey)
		if err != nil {
			return err
		}
		if _, err := st.CreateProvider(ctx, store.Provider{
			Name: "Primary", Kind: upstream.KindOpenAI, BaseURL: cfg.OpenRouterURL, APIKeyEnc: enc, IsOpenRouter: true, Enabled: true,
		}); err != nil {
			return err
		}
		log.Info("created default upstream provider")
	}
	if len(cfg.AdminEmails) > 0 {
		if _, err := st.Pool.Exec(ctx, `UPDATE users SET role='admin' WHERE email = ANY($1) AND role <> 'admin'`, cfg.AdminEmails); err != nil {
			return err
		}
	}
	return nil
}

// Run starts background workers and serves HTTP until ctx is canceled.
func (a *App) Run(ctx context.Context) error {
	bg, cancel := context.WithCancel(context.Background())
	defer cancel()

	go a.Ledger.Run()
	go a.Catalog.Run(bg, a.Cfg.CatalogRefresh)
	go a.Catalog.RunSync(bg, a.Cfg.ModelSyncEvery)
	go a.Bus.Listen(bg)
	go every(bg, time.Minute, func() { a.Limiter.Sweep(10 * time.Minute) })
	retention := 90
	if v, err := strconv.Atoi(os.Getenv("USAGE_RETENTION_DAYS")); err == nil && v > 0 {
		retention = v
	}
	go every(bg, time.Hour, func() {
		if n, err := a.Store.PruneUsage(bg, time.Duration(retention)*24*time.Hour); err != nil {
			a.Log.Warn("usage prune failed", "err", err)
		} else if n > 0 {
			a.Log.Info("pruned usage logs", "rows", n)
		}
	})

	srv := &http.Server{
		Addr:              a.Cfg.Addr,
		Handler:           a.Handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
		// No WriteTimeout: responses stream for as long as the model generates.
	}
	errCh := make(chan error, 1)
	go func() {
		a.Log.Info("listening", "addr", a.Cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			return err
		}
	}
	a.Log.Info("shutting down; draining in-flight requests")
	sctx, scancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer scancel()
	_ = srv.Shutdown(sctx)
	cancel()
	a.Ledger.Stop() // final flush of usage
	a.Pool.Close()
	return nil
}

func every(ctx context.Context, d time.Duration, fn func()) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fn()
		}
	}
}
