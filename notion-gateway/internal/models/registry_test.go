package models_test

import (
	"context"
	"github.com/notioncodex/gateway/internal/models"
	"github.com/notioncodex/gateway/internal/notion/transport/mock"
	"testing"
)

func TestRegistryRefresh(t *testing.T) {
	r := models.NewRegistry(&mock.Transport{})
	if r.Ready() {
		t.Fatal("new registry must not be ready")
	}
	if err := r.Refresh(context.Background(), "a", "w"); err != nil {
		t.Fatal(err)
	}
	got := r.List()
	if len(got) != 1 || got[0].PublicID != mock.ModelID {
		t.Fatalf("got %#v", got)
	}
}
