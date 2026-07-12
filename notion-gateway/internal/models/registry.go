package models

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/notioncodex/gateway/internal/domain"
	"github.com/notioncodex/gateway/internal/ports"
)

type Registry struct {
	mu         sync.RWMutex
	models     map[string]domain.Model
	discoverer ports.ModelDiscoverer
}

func NewRegistry(d ports.ModelDiscoverer) *Registry {
	return &Registry{models: make(map[string]domain.Model), discoverer: d}
}
func (r *Registry) Refresh(ctx context.Context, accountID, workspaceID string) error {
	found, err := r.discoverer.Discover(ctx, accountID, workspaceID)
	if err != nil {
		return err
	}
	next := make(map[string]domain.Model, len(found))
	for _, m := range found {
		if m.PublicID == "" || m.UpstreamID == "" {
			return errors.New("discovery returned incomplete model")
		}
		next[m.PublicID] = m
	}
	r.mu.Lock()
	r.models = next
	r.mu.Unlock()
	return nil
}
func (r *Registry) List() []domain.Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.Model, 0, len(r.models))
	for _, m := range r.models {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublicID < out[j].PublicID })
	return out
}
func (r *Registry) Ready() bool { r.mu.RLock(); defer r.mu.RUnlock(); return len(r.models) > 0 }
