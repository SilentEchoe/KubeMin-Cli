package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

// NodeSelectorProfileRepository defines the interface for node selector profile data operations.
type NodeSelectorProfileRepository interface {
	FindByID(ctx context.Context, id string) (*model.NodeSelectorProfile, error)
	FindByName(ctx context.Context, name string) (*model.NodeSelectorProfile, error)
	Create(ctx context.Context, profile *model.NodeSelectorProfile) error
	Update(ctx context.Context, profile *model.NodeSelectorProfile) error
	Delete(ctx context.Context, profile *model.NodeSelectorProfile) error
	List(ctx context.Context, options datastore.ListOptions) ([]*model.NodeSelectorProfile, error)
}

type nodeSelectorProfileRepository struct {
	Store datastore.DataStore `inject:"datastore"`
}

// NewNodeSelectorProfileRepository creates a new NodeSelectorProfileRepository.
// Dependencies are injected via struct tags.
func NewNodeSelectorProfileRepository() NodeSelectorProfileRepository {
	return &nodeSelectorProfileRepository{}
}

func (r *nodeSelectorProfileRepository) FindByID(ctx context.Context, id string) (*model.NodeSelectorProfile, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, datastore.ErrPrimaryEmpty
	}
	profile := &model.NodeSelectorProfile{ID: id}
	if err := r.Store.Get(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func (r *nodeSelectorProfileRepository) FindByName(ctx context.Context, name string) (*model.NodeSelectorProfile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("node selector profile name is empty")
	}
	items, err := r.Store.List(ctx, &model.NodeSelectorProfile{Name: name}, &datastore.ListOptions{Page: 1, PageSize: 1})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, datastore.ErrRecordNotExist
	}
	profile, ok := items[0].(*model.NodeSelectorProfile)
	if !ok || profile == nil {
		return nil, datastore.ErrEntityInvalid
	}
	return profile, nil
}

// Create creates a profile; if the profile name already exists, it updates that record (upsert-by-name).
func (r *nodeSelectorProfileRepository) Create(ctx context.Context, profile *model.NodeSelectorProfile) error {
	if profile == nil {
		return datastore.ErrNilEntity
	}
	if err := r.Store.Add(ctx, profile); err != nil {
		if !errors.Is(err, datastore.ErrRecordExist) {
			return err
		}
		if profile.Name == "" {
			return fmt.Errorf("node selector profile name is empty: %w", err)
		}
		existing, findErr := r.FindByName(ctx, profile.Name)
		if findErr != nil {
			return err
		}
		profile.ID = existing.ID
		return r.Store.Put(ctx, profile)
	}
	return nil
}

func (r *nodeSelectorProfileRepository) Update(ctx context.Context, profile *model.NodeSelectorProfile) error {
	return r.Store.Put(ctx, profile)
}

func (r *nodeSelectorProfileRepository) Delete(ctx context.Context, profile *model.NodeSelectorProfile) error {
	return r.Store.Delete(ctx, profile)
}

func (r *nodeSelectorProfileRepository) List(ctx context.Context, options datastore.ListOptions) ([]*model.NodeSelectorProfile, error) {
	var query model.NodeSelectorProfile
	entities, err := r.Store.List(ctx, &query, &options)
	if err != nil {
		return nil, err
	}
	out := make([]*model.NodeSelectorProfile, 0, len(entities))
	for _, entity := range entities {
		profile, ok := entity.(*model.NodeSelectorProfile)
		if !ok || profile == nil {
			continue
		}
		out = append(out, profile)
	}
	return out, nil
}
