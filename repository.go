package mochi

import (
	"context"
	"fmt"
)

type Repository[M Model] interface {
	FindOne(ctx context.Context, query string, args ...interface{}) (M, error)
	FindOneByID(ctx context.Context, itemID uint, query string, args ...interface{}) (M, error)
	FindOneByUser(ctx context.Context, userID uint, query string, args ...interface{}) (M, error)
	FindManyByUser(ctx context.Context, userID uint, query string, args ...interface{}) ([]M, error)
	CreateOne(ctx context.Context, item M) error
	UpdateOne(ctx context.Context, itemID uint, item M) error
	DeleteOne(ctx context.Context, itemID uint) error
}

type repository[M Model] struct {
	db     DBService
	logger LoggerService

	joinTables    []string
	preloadTables []string
	tableName     string
}

type RepositoryOption[M Model] func(*repository[M])

func NewRepository[M Model](
	db DBService,
	logger LoggerService,
	opts ...RepositoryOption[M],
) Repository[M] {
	repo := &repository[M]{
		db:     db,
		logger: logger,
	}

	for _, opt := range opts {
		opt(repo)
	}

	return repo
}

func (r *repository[M]) FindOne(ctx context.Context, query string, args ...interface{}) (M, error) {
	var item M

	err := r.db.FindOne(ctx, &item, r.joinTables, []string{}, query, args...)
	if err != nil {
		return item, fmt.Errorf("failed to find one item: %w", err)
	}

	r.logger.Debug("Found one item", "item", item.GetID(), "table", r.tableName)

	return item, nil
}

func (r *repository[M]) FindOneByID(ctx context.Context, itemID uint, query string, args ...interface{}) (M, error) {
	fullQuery := fmt.Sprintf("%s.id = ?", r.tableName)
	if query != "" {
		fullQuery = fmt.Sprintf("%s AND %s", fullQuery, query)
	}

	fullArgs := append([]interface{}{itemID}, args...)

	return r.FindOne(ctx, fullQuery, fullArgs...)
}

func (r *repository[M]) FindOneByUser(ctx context.Context, userID uint, query string, args ...interface{}) (M, error) {
	var item M

	fullQuery := fmt.Sprintf("%s.user_id = ?", r.tableName)
	if query != "" {
		fullQuery = fmt.Sprintf("%s AND %s", fullQuery, query)
	}

	fullArgs := append([]interface{}{userID}, args...)

	err := r.db.FindOne(ctx, &item, r.joinTables, []string{}, fullQuery, fullArgs...)
	if err != nil {
		return item, fmt.Errorf("failed to find one item: %w", err)
	}

	r.logger.Debug("Found one item by user", "item", item.GetID(), "table", r.tableName)

	return item, nil
}

func (r *repository[M]) FindManyByUser(ctx context.Context, userID uint, query string, args ...interface{}) ([]M, error) {
	var items []M

	fullQuery := fmt.Sprintf("%s.user_id = ?", r.tableName)
	if query != "" {
		fullQuery = fmt.Sprintf("%s AND %s", fullQuery, query)
	}

	fullArgs := append([]interface{}{userID}, args...)

	err := r.db.FindMany(ctx, &items, r.joinTables, r.preloadTables, fullQuery, fullArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to find many items by user: %w", err)
	}

	r.logger.Debug("Found many items by user", "table", r.tableName, "count", len(items))

	return items, nil
}

func (r *repository[M]) CreateOne(ctx context.Context, item M) error {
	err := r.db.CreateOne(ctx, item)
	if err != nil {
		return fmt.Errorf("failed to create one item: %w", err)
	}

	r.logger.Debug("Created one item", "item", item.GetID(), "table", r.tableName)

	return nil
}

func (r *repository[M]) UpdateOne(ctx context.Context, itemID uint, item M) error {
	err := r.db.UpdateOne(ctx, itemID, item)
	if err != nil {
		return fmt.Errorf("failed to update one item: %w", err)
	}

	r.logger.Debug("Updated one item", "item", item.GetID(), "table", r.tableName)

	return nil
}

func (r *repository[M]) DeleteOne(ctx context.Context, itemID uint) error {
	item := new(M)

	err := r.db.DeleteOne(ctx, itemID, item)
	if err != nil {
		return fmt.Errorf("failed to delete one item: %w", err)
	}

	r.logger.Debug("Deleted one item", "item", itemID, "table", r.tableName)

	return nil
}

func WithTableName[M Model](tableName string) RepositoryOption[M] {
	return func(r *repository[M]) {
		r.tableName = tableName
	}
}

func WithJoinTables[M Model](joinTables ...string) RepositoryOption[M] {
	return func(r *repository[M]) {
		r.joinTables = joinTables
	}
}

func WithPreloadTables[M Model](preloadTables ...string) RepositoryOption[M] {
	return func(r *repository[M]) {
		r.preloadTables = preloadTables
	}
}
