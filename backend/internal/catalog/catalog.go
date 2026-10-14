// Package catalog keeps an immutable in-memory snapshot of the public model
// catalog, providers and platform settings so the hot path never hits the DB.
package catalog

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ronoksheikh/modelrouter.app/backend/internal/billing"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/secret"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/store"
	"github.com/ronoksheikh/modelrouter.app/backend/internal/upstream"
)

type Entry struct {
	Model    store.Model
	Provider *upstream.Provider
	Price    billing.Price
	// UpstreamPrice is what the upstream charges us (for profit reporting).
	UpstreamPrice billing.Price
}

func (e *Entry) IsFree() bool { return e.Price.InputPerM == 0 && e.Price.OutputPerM == 0 }

type Snapshot struct {
	Settings  store.Settings
	Providers map[int64]*upstream.Provider
	Models    []*Entry // enabled models in display order
	bySlug    map[string]*Entry
	byLower   map[string]*Entry
	LoadedAt  time.Time
}

func (s *Snapshot) Lookup(slug string) (*Entry, bool) {
	if e, ok := s.bySlug[slug]; ok {
		return e, true
	}
	e, ok := s.byLower[strings.ToLower(slug)]
	return e, ok
}

// Build assembles a snapshot from raw rows. Exported for tests.
func Build(settings store.Settings, providers []*upstream.Provider, models []store.Model) *Snapshot {
	s := &Snapshot{
		Settings:  settings,
		Providers: make(map[int64]*upstream.Provider, len(providers)),
		bySlug:    make(map[string]*Entry, len(models)),
		byLower:   make(map[string]*Entry, len(models)),
		LoadedAt:  time.Now(),
	}
	for _, p := range providers {
		s.Providers[p.ID] = p
	}
	for _, m := range models {
		if !m.Enabled {
			continue
		}
		p, ok := s.Providers[m.ProviderID]
		if !ok {
			continue // provider disabled or missing
		}
		fee := settings.DefaultRequestFee
		if m.RequestFee != nil {
			fee = *m.RequestFee
		}
		if m.MaxOutputTokens == 0 && settings.DefaultMaxOutputTokens > 0 {
			m.MaxOutputTokens = settings.DefaultMaxOutputTokens
		}
		e := &Entry{
			Model:         m,
			Provider:      p,
			Price:         billing.Price{InputPerM: m.InputPrice, OutputPerM: m.OutputPrice, RequestFee: fee},
			UpstreamPrice: billing.Price{InputPerM: m.UpstreamInputPrice, OutputPerM: m.UpstreamOutputPrice},
		}
		s.Models = append(s.Models, e)
		s.bySlug[m.Slug] = e
		s.byLower[strings.ToLower(m.Slug)] = e
	}
	return s
}

type Catalog struct {
	st   *store.Store
	box  *secret.Box
	log  *slog.Logger
	cur  atomic.Pointer[Snapshot]
	http *upstream.Client
}

func New(st *store.Store, box *secret.Box, client *upstream.Client, log *slog.Logger) *Catalog {
	c := &Catalog{st: st, box: box, log: log, http: client}
	c.cur.Store(Build(store.DefaultSettings(), nil, nil))
	return c
}

// NewStatic returns a catalog serving a fixed snapshot (tests).
func NewStatic(s *Snapshot) *Catalog {
	c := &Catalog{}
	c.cur.Store(s)
	return c
}

func (c *Catalog) Snapshot() *Snapshot { return c.cur.Load() }

func (c *Catalog) Reload(ctx context.Context) error {
	settings, err := c.st.GetSettings(ctx)
	if err != nil {
		return err
	}
	rows, err := c.st.ListProviders(ctx)
	if err != nil {
		return err
	}
	var providers []*upstream.Provider
	for _, r := range rows {
		if !r.Enabled {
			continue
		}
		key, err := c.box.Decrypt(r.APIKeyEnc)
		if err != nil {
			c.log.Error("decrypt provider key", "provider", r.Name, "err", err)
			continue
		}
		providers = append(providers, &upstream.Provider{
			ID: r.ID, Name: r.Name, Kind: r.Kind, BaseURL: r.BaseURL, APIKey: key, Headers: r.Headers, IsOpenRouter: r.IsOpenRouter,
		})
	}
	models, err := c.st.ListModels(ctx, false)
	if err != nil {
		return err
	}
	c.cur.Store(Build(settings, providers, models))
	return nil
}

// Run reloads the snapshot periodically until ctx is done.
func (c *Catalog) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := c.Reload(ctx); err != nil && ctx.Err() == nil {
				c.log.Warn("catalog reload failed", "err", err)
			}
		}
	}
}

// SyncResult summarizes a provider catalog sync.
type SyncResult struct {
	ProviderID int64    `json:"provider_id"`
	Fetched    int      `json:"fetched"`
	Added      []string `json:"added"`
	Published  int64    `json:"published"`
	Repriced   int64    `json:"repriced"`
}

// SyncProvider pulls the upstream catalog, stores it, optionally publishes new
// models and re-prices auto-priced models.
func (c *Catalog) SyncProvider(ctx context.Context, providerID int64) (*SyncResult, error) {
	snap := c.Snapshot()
	p, ok := snap.Providers[providerID]
	if !ok {
		if err := c.Reload(ctx); err != nil {
			return nil, err
		}
		if p, ok = c.Snapshot().Providers[providerID]; !ok {
			return nil, fmt.Errorf("provider %d not found or disabled", providerID)
		}
	}
	models, err := c.http.FetchModels(ctx, p)
	if err != nil {
		return nil, err
	}
	added, err := c.st.UpsertUpstreamModels(ctx, providerID, models)
	if err != nil {
		return nil, err
	}
	res := &SyncResult{ProviderID: providerID, Fetched: len(models), Added: added}
	settings, err := c.st.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if settings.AutoPublishNewModels && len(added) > 0 {
		if res.Published, err = c.st.PublishUpstream(ctx, providerID, added, nil, true); err != nil {
			return nil, err
		}
	}
	if res.Repriced, err = c.st.RepriceAuto(ctx, settings.DefaultMarkupBps); err != nil {
		return nil, err
	}
	sort.Strings(res.Added)
	return res, c.Reload(ctx)
}

// RunSync syncs all enabled providers periodically.
func (c *Catalog) RunSync(ctx context.Context, every time.Duration) {
	sync := func() {
		for id, p := range c.Snapshot().Providers {
			res, err := c.SyncProvider(ctx, id)
			if err != nil {
				if ctx.Err() == nil {
					c.log.Warn("model sync failed", "provider", p.Name, "err", err)
				}
				continue
			}
			c.log.Info("model sync", "provider", p.Name, "fetched", res.Fetched, "added", len(res.Added), "repriced", res.Repriced)
		}
	}
	sync()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sync()
		}
	}
}
