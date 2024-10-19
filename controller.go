package mochi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/burkel24/go-mochi/internal"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type ResourceContextKey int

type ResourceRequestConstructor[M internal.Resource] func(r *http.Request) (M, error)

type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

type Controller[M internal.Resource] struct {
	additionalDetailRoutes []Route
	contextKey             ResourceContextKey

	auth   internal.AuthService
	logger internal.LoggerService
	svc    internal.Service[M]
	Router *chi.Mux

	createRequestConstructor ResourceRequestConstructor[M]
	updateRequestConstructor ResourceRequestConstructor[M]
}

type ControllerOption[M internal.Resource] func(*Controller[M])

func NewController[M internal.Resource](
	svc internal.Service[M],
	logger internal.LoggerService,
	authSvc internal.AuthService,
	createRequestConstructor ResourceRequestConstructor[M],
	updateRequestConstructor ResourceRequestConstructor[M],
	opts ...ControllerOption[M],
) internal.Controller[M] {
	ctrl := &Controller[M]{
		additionalDetailRoutes: make([]Route, 0),

		auth:   authSvc,
		logger: logger,
		svc:    svc,

		createRequestConstructor: createRequestConstructor,
		updateRequestConstructor: updateRequestConstructor,
	}

	for _, opt := range opts {
		opt(ctrl)
	}

	ctrl.Router = chi.NewRouter()
	ctrl.Router.Use(authSvc.AuthRequired())

	ctrl.Router.Get("/", ctrl.List)
	ctrl.Router.Post("/", ctrl.Create)

	ctrl.Router.Route("/{id}", func(r chi.Router) {
		r.Use(ctrl.ItemContextMiddleware)
		r.Use(ctrl.UserAccessMiddleware)

		r.Get("/", ctrl.Get)
		r.Patch("/", ctrl.Update)
		r.Delete("/", ctrl.Delete)

		for _, route := range ctrl.additionalDetailRoutes {
			r.Method(route.Method, route.Path, route.Handler)
		}
	})

	return ctrl
}

func (c *Controller[M]) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, err := c.auth.GetUserFromCtx(ctx)
	if err != nil {
		render.Render(w, r, ErrUnauthorized(err))
		return
	}

	items, err := c.svc.ListByUser(ctx, user.ID())
	if err != nil {
		c.logger.Error("failed to list items", "error", err)
		render.Render(w, r, ErrUnknown(err))

		return
	}

	respList := []render.Renderer{}
	for _, item := range items {
		respList = append(respList, item.ToDTO())
	}

	render.RenderList(w, r, respList)
}

func (c *Controller[M]) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, err := c.auth.GetUserFromCtx(ctx)
	if err != nil {
		render.Render(w, r, ErrUnauthorized(err))
		return
	}

	newItem, err := c.createRequestConstructor(r)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	item, err := c.svc.CreateOne(ctx, user.ID(), newItem)
	if err != nil {
		c.logger.Error("failed to create item", "error", err)
		render.Render(w, r, ErrUnknown(err))

		return
	}

	render.Status(r, http.StatusCreated)
	render.Render(w, r, item.ToDTO())
}

func (c *Controller[M]) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	item, err := c.ItemFromContext(ctx)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	render.Render(w, r, item.ToDTO())
}

func (c *Controller[M]) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	item, err := c.ItemFromContext(ctx)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	update, err := c.updateRequestConstructor(r)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	updatedItem, err := c.svc.UpdateOne(ctx, item.GetID(), update)
	if err != nil {
		c.logger.Error("failed to update item", "error", err)
		render.Render(w, r, ErrUnknown(err))

		return
	}

	render.Render(w, r, updatedItem.ToDTO())
}

func (c *Controller[M]) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	item, err := c.ItemFromContext(ctx)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	err = c.svc.DeleteOne(ctx, item.GetID())
	if err != nil {
		c.logger.Error("failed to delete item", "error", err)
		render.Render(w, r, ErrUnknown(err))

		return
	}

	render.NoContent(w, r)
}

func (c *Controller[M]) ItemFromContext(ctx context.Context) (M, error) {
	var item M

	item, ok := ctx.Value(c.contextKey).(M)
	if !ok {
		return item, fmt.Errorf("failed to get item from context")
	}

	return item, nil
}

func (c *Controller[M]) ItemContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		itemID := chi.URLParam(r, "id")
		if itemID == "" {
			render.Render(w, r, ErrNotFound)
			return
		}

		itemIDInt, err := strconv.Atoi(itemID)
		if err != nil {
			render.Render(w, r, ErrInvalidRequest(fmt.Errorf("failed to parse ID: %w", err)))
			return
		}

		item, err := c.svc.GetOne(ctx, uint(itemIDInt))
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				render.Render(w, r, ErrNotFound)
			} else {
				c.logger.Error("failed to look up item", "error", err)
				render.Render(w, r, ErrUnknown(err))
			}

			return
		}

		ctxWithTask := context.WithValue(r.Context(), c.contextKey, item)

		next.ServeHTTP(w, r.WithContext(ctxWithTask))
	})
}

func (c *Controller[M]) UserAccessMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		user, err := c.auth.GetUserFromCtx(ctx)
		if err != nil {
			render.Render(w, r, ErrUnauthorized(err))
			return
		}

		item, err := c.ItemFromContext(ctx)
		if err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		if item.GetUserID() != user.ID() {
			render.Render(w, r, ErrNotFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (c *Controller[M]) GetRouter() *chi.Mux {
	return c.Router
}

func WithDetailRoute[M internal.Resource](method, path string, handler http.HandlerFunc) ControllerOption[M] {
	return func(c *Controller[M]) {
		c.additionalDetailRoutes = append(c.additionalDetailRoutes, Route{
			Method:  method,
			Path:    path,
			Handler: handler,
		})
	}
}

func WithContextKey[M internal.Resource](key ResourceContextKey) ControllerOption[M] {
	return func(c *Controller[M]) {
		c.contextKey = key
	}
}
