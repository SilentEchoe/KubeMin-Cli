package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

// RBACProfileRepository defines the interface for RBAC profile data operations.
type RBACProfileRepository interface {
	FindByID(ctx context.Context, id string) (*model.RBACProfile, error)
	FindByName(ctx context.Context, name string) (*model.RBACProfile, error)
	Create(ctx context.Context, profile *model.RBACProfile) error
	Update(ctx context.Context, profile *model.RBACProfile) error
	Delete(ctx context.Context, profile *model.RBACProfile) error
	List(ctx context.Context, options datastore.ListOptions) ([]*model.RBACProfile, error)
}

type rbacProfileRepository struct {
	Store datastore.DataStore `inject:"datastore"`
}

// NewRBACProfileRepository creates a new RBACProfileRepository.
// Dependencies are injected via struct tags.
func NewRBACProfileRepository() RBACProfileRepository {
	return &rbacProfileRepository{}
}

func (r *rbacProfileRepository) FindByID(ctx context.Context, id string) (*model.RBACProfile, error) {
	if strings.TrimSpace(id) == "" {
		return nil, datastore.ErrPrimaryEmpty
	}
	profile := &model.RBACProfile{ID: id}
	if err := r.Store.Get(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func (r *rbacProfileRepository) FindByName(ctx context.Context, name string) (*model.RBACProfile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("rbac profile name is empty")
	}
	items, err := r.Store.List(ctx, &model.RBACProfile{Name: name}, &datastore.ListOptions{Page: 1, PageSize: 1})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, datastore.ErrRecordNotExist
	}
	profile, ok := items[0].(*model.RBACProfile)
	if !ok || profile == nil {
		return nil, datastore.ErrEntityInvalid
	}
	return profile, nil
}

// Create creates a profile; if the profile name already exists, it updates that record (upsert-by-name).
func (r *rbacProfileRepository) Create(ctx context.Context, profile *model.RBACProfile) error {
	if profile == nil {
		return datastore.ErrNilEntity
	}
	if err := r.Store.Add(ctx, profile); err != nil {
		if !errors.Is(err, datastore.ErrRecordExist) {
			return err
		}
		if profile.Name == "" {
			return fmt.Errorf("rbac profile name is empty: %w", err)
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

func (r *rbacProfileRepository) Update(ctx context.Context, profile *model.RBACProfile) error {
	return r.Store.Put(ctx, profile)
}

func (r *rbacProfileRepository) Delete(ctx context.Context, profile *model.RBACProfile) error {
	return r.Store.Delete(ctx, profile)
}

func (r *rbacProfileRepository) List(ctx context.Context, options datastore.ListOptions) ([]*model.RBACProfile, error) {
	var query model.RBACProfile
	entities, err := r.Store.List(ctx, &query, &options)
	if err != nil {
		return nil, err
	}
	out := make([]*model.RBACProfile, 0, len(entities))
	for _, entity := range entities {
		profile, ok := entity.(*model.RBACProfile)
		if !ok || profile == nil {
			continue
		}
		out = append(out, profile)
	}
	return out, nil
}
