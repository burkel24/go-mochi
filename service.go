package mochi

import (
	"context"
	"fmt"
)

type ServiceQuery struct {
	Filter string
	Args   []interface{}
}

type Service[M Resource] interface {
	ListByUser(ctx context.Context, userID uint) ([]M, error)
	CreateOne(ctx context.Context, userID uint, item M) (M, error)
	GetOne(ctx context.Context, itemID uint) (M, error)
	UpdateOne(ctx context.Context, itemID uint, item M) (M, error)
	DeleteOne(ctx context.Context, itemID uint) error
}

type service[M Resource] struct {
	repo Repository[M]

	listQuery *ServiceQuery
	getQuery  *ServiceQuery
}

type ServiceOption[M Resource] func(*service[M])

func NewService[M Resource](
	repo Repository[M],
	opts ...ServiceOption[M],
) Service[M] {
	svc := &service[M]{
		repo: repo,
	}

	for _, opt := range opts {
		opt(svc)
	}

	if svc.listQuery == nil {
		svc.listQuery = &ServiceQuery{}
	}

	if svc.getQuery == nil {
		svc.getQuery = &ServiceQuery{}
	}

	return svc
}

func (s *service[M]) ListByUser(ctx context.Context, userID uint) ([]M, error) {
	items, err := s.repo.FindManyByUser(ctx, userID, s.listQuery.Filter, s.listQuery.Args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list user items: %w", err)
	}

	return items, nil
}

func (s *service[M]) CreateOne(ctx context.Context, userID uint, item M) (M, error) {
	err := s.repo.CreateOne(ctx, item)
	if err != nil {
		return item, fmt.Errorf("failed to create user task: %w", err)
	}

	return item, nil
}

func (s *service[M]) GetOne(ctx context.Context, itemID uint) (M, error) {
	item, err := s.repo.FindOneByID(ctx, itemID, s.getQuery.Filter, s.getQuery.Args...)
	if err != nil {
		return item, fmt.Errorf("failed to get item: %w", err)
	}

	return item, nil
}

func (s *service[M]) UpdateOne(ctx context.Context, itemID uint, item M) (M, error) {
	err := s.repo.UpdateOne(ctx, itemID, item)
	if err != nil {
		return item, fmt.Errorf("failed to update user task: %w", err)
	}

	return item, nil
}

func (s *service[M]) DeleteOne(ctx context.Context, itemID uint) error {
	err := s.repo.DeleteOne(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to delete user task: %w", err)
	}

	return nil
}

func WithListQuery[M Resource](query string, args ...interface{}) ServiceOption[M] {
	return func(s *service[M]) {
		s.listQuery = &ServiceQuery{
			Filter: query,
			Args:   args,
		}
	}
}

func WithGetQuery[M Resource](query string, args ...interface{}) ServiceOption[M] {
	return func(s *service[M]) {
		s.getQuery = &ServiceQuery{
			Filter: query,
			Args:   args,
		}
	}
}
