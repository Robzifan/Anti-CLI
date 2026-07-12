// Package picker contains the version-scoped adapter for the observed Notion
// model-picker contract. No other package should depend on its endpoint or DTOs.
package picker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/notioncodex/gateway/internal/domain"
)

const (
	ObservedPath     = "/api/v3/getAvailableModels"
	DiscoverySource  = "notion_model_picker_observed_2026_07"
	maxResponseBytes = 2 << 20
)

type Request struct {
	Method string
	Path   string
	Body   []byte
}

type Session interface {
	Do(context.Context, Request) ([]byte, error)
}

type Discoverer struct {
	Session Session
	Now     func() time.Time
}

func (d *Discoverer) Discover(ctx context.Context, accountID, workspaceID string) ([]domain.Model, error) {
	if d.Session == nil {
		return nil, errors.New("model discovery session is required")
	}
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(workspaceID) == "" {
		return nil, errors.New("account and workspace IDs are required")
	}
	body, err := json.Marshal(struct {
		SpaceID string `json:"spaceId"`
	}{SpaceID: workspaceID})
	if err != nil {
		return nil, fmt.Errorf("encode model discovery request: %w", err)
	}
	response, err := d.Session.Do(ctx, Request{Method: http.MethodPost, Path: ObservedPath, Body: body})
	if err != nil {
		return nil, fmt.Errorf("execute model discovery: %w", err)
	}
	if len(response) > maxResponseBytes {
		return nil, errors.New("model discovery response exceeds limit")
	}
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now().UTC()
	}
	return Parse(response, accountID, workspaceID, now)
}

type pickerResponse struct {
	Restricted []restrictedModel `json:"restrictedAccessModelsInPickerConfig"`
	Models     []pickerModel     `json:"models"`
}

type restrictedModel struct {
	Codename string `json:"codename"`
}

type pickerModel struct {
	Model                    string             `json:"model"`
	ModelMessage             string             `json:"modelMessage"`
	ModelFamily              string             `json:"modelFamily"`
	DisplayGroup             string             `json:"displayGroup"`
	ModelConfiguration       modelConfiguration `json:"modelConfiguration"`
	IsDisabled               bool               `json:"isDisabled"`
	IsApproachingRateLimit   *bool              `json:"isApproachingRateLimit"`
	RestrictedAccessCodename string             `json:"restrictedAccessModelCodename"`
	Workflow                 *routeConfig       `json:"workflow"`
	CustomAgent              *routeConfig       `json:"customAgent"`
	AgentService             *routeConfig       `json:"agentService"`
}

type modelConfiguration struct {
	SupportedReasoningEfforts []string `json:"supportedReasoningEfforts"`
	DefaultReasoningEffort    string   `json:"defaultReasoningEffort"`
}

type routeConfig struct {
	FinalModelName string `json:"finalModelName"`
	Beta           bool   `json:"beta"`
}

func Parse(payload []byte, accountID, workspaceID string, observedAt time.Time) ([]domain.Model, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	var response pickerResponse
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode observed model picker response: %w", err)
	}
	if len(response.Models) == 0 {
		return nil, errors.New("observed model picker response contains no models")
	}
	restricted := make(map[string]struct{}, len(response.Restricted))
	for _, item := range response.Restricted {
		if item.Codename != "" {
			restricted[item.Codename] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(response.Models))
	models := make([]domain.Model, 0, len(response.Models))
	for index, item := range response.Models {
		if strings.TrimSpace(item.Model) == "" || strings.TrimSpace(item.ModelMessage) == "" {
			return nil, fmt.Errorf("model %d is missing model or modelMessage", index)
		}
		if _, exists := seen[item.Model]; exists {
			return nil, fmt.Errorf("duplicate upstream model %q", item.Model)
		}
		seen[item.Model] = struct{}{}
		status := "available"
		if item.IsDisabled {
			status = "disabled"
		}
		routes := make([]string, 0, 3)
		if item.Workflow != nil {
			routes = append(routes, "workflow")
		}
		if item.CustomAgent != nil {
			routes = append(routes, "custom_agent")
		}
		if item.AgentService != nil {
			routes = append(routes, "agent_service")
		}
		diagnostic := ""
		if item.ModelFamily == "mystery" {
			diagnostic = "upstream reported provider family as mystery"
		}
		if item.RestrictedAccessCodename != "" {
			if _, configured := restricted[item.RestrictedAccessCodename]; !configured {
				diagnostic = joinDiagnostic(diagnostic, "restricted access codename missing from picker config")
			}
		}
		models = append(models, domain.Model{
			PublicID:     "notion/" + item.Model,
			UpstreamID:   item.Model,
			DisplayName:  item.ModelMessage,
			Provider:     item.ModelFamily,
			AccountID:    accountID,
			WorkspaceID:  workspaceID,
			Capabilities: domain.Capabilities{Text: true},
			CapabilitySnapshot: domain.CapabilitySnapshot{
				SupportedReasoningEfforts: append([]string(nil), item.ModelConfiguration.SupportedReasoningEfforts...),
				DefaultReasoningEffort:    item.ModelConfiguration.DefaultReasoningEffort,
				DisplayGroup:              item.DisplayGroup,
				Routes:                    routes,
				ApproachingRateLimit:      item.IsApproachingRateLimit,
				RestrictedAccessCodename:  item.RestrictedAccessCodename,
			},
			Status:          status,
			LastVerifiedAt:  observedAt.UTC(),
			DiscoverySource: DiscoverySource,
			Diagnostic:      diagnostic,
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].PublicID < models[j].PublicID })
	return models, nil
}

func joinDiagnostic(existing, added string) string {
	if existing == "" {
		return added
	}
	return existing + "; " + added
}
