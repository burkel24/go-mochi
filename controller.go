package mochi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type ResourceContextKey int

type ResourceRequestConstructor[M Resource] func(*http.Request, User) (M, error)

type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

type Controller[M Resource] interface {
	List(w http.ResponseWriter, r *http.Request)
	Create(w http.ResponseWriter, r *http.Request)
	Get(w http.ResponseWriter, r *http.Request)
	Update(w http.ResponseWriter, r *http.Request)
	Delete(w http.ResponseWriter, r *http.Request)

	ItemFromContext(ctx context.Context) (M, error)
	ItemContextMiddleware(next http.Handler) http.Handler
	UserAccessMiddleware(next http.Handler) http.Handler

	GetRouter() *chi.Mux
}

type UserResourceAccessFunc[M Resource] func(User, M) error

func defaultUserResourceAccessFunc[M Resource](u User, item M) error {
	return fmt.Errorf("user access func not implemented")
}

type controller[M Resource] struct {
	additionalDetailRoutes []Route
	contextKey             ResourceContextKey

	auth   AuthService
	logger LoggerService
	svc    Service[M]
	Router *chi.Mux

	userAccessFunc UserResourceAccessFunc[M]

	createRequestConstructor ResourceRequestConstructor[M]
	updateRequestConstructor ResourceRequestConstructor[M]
}

type ControllerOption[M Resource] func(*controller[M])

func NewController[M Resource](
	svc Service[M],
	logger LoggerService,
	authSvc AuthService,
	createRequestConstructor ResourceRequestConstructor[M],
	updateRequestConstructor ResourceRequestConstructor[M],
	opts ...ControllerOption[M],
) Controller[M] {
	ctrl := &controller[M]{
		additionalDetailRoutes: make([]Route, 0),

		auth:   authSvc,
		logger: logger,
		svc:    svc,

		userAccessFunc: defaultUserResourceAccessFunc[M],

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

func (c *controller[M]) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, err := c.auth.GetUserFromCtx(ctx)
	if err != nil {
		render.Render(w, r, ErrUnauthorized(err))
		return
	}

	items, err := c.svc.ListByUser(ctx, user.GetID())
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

func (c *controller[M]) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, err := c.auth.GetUserFromCtx(ctx)
	if err != nil {
		render.Render(w, r, ErrUnauthorized(err))
		return
	}

	newItem, err := c.createRequestConstructor(r, user)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	item, err := c.svc.CreateOne(ctx, user.GetID(), newItem)
	if err != nil {
		c.logger.Error("failed to create item", "error", err)
		render.Render(w, r, ErrUnknown(err))

		return
	}

	render.Status(r, http.StatusCreated)
	render.Render(w, r, item.ToDTO())
}

func (c *controller[M]) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	item, err := c.ItemFromContext(ctx)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	render.Render(w, r, item.ToDTO())
}

func (c *controller[M]) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// user is already checked in UserAccessMiddleware so we can safely ignore the error
	user, _ := c.auth.GetUserFromCtx(ctx)

	item, err := c.ItemFromContext(ctx)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	update, err := c.updateRequestConstructor(r, user)
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

func (c *controller[M]) Delete(w http.ResponseWriter, r *http.Request) {
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

func (c *controller[M]) ItemFromContext(ctx context.Context) (M, error) {
	var item M

	item, ok := ctx.Value(c.contextKey).(M)
	if !ok {
		return item, fmt.Errorf("failed to get item from context")
	}

	return item, nil
}

func (c *controller[M]) ItemContextMiddleware(next http.Handler) http.Handler {
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

func (c *controller[M]) UserAccessMiddleware(next http.Handler) http.Handler {
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

		accessErr := c.userAccessFunc(user, item)
		if accessErr != nil {
			render.Render(w, r, ErrNotFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (c *controller[M]) GetRouter() *chi.Mux {
	return c.Router
}

func WithDetailRoute[M Resource](method, path string, handler http.HandlerFunc) ControllerOption[M] {
	return func(c *controller[M]) {
		c.additionalDetailRoutes = append(c.additionalDetailRoutes, Route{
			Method:  method,
			Path:    path,
			Handler: handler,
		})
	}
}

func WithContextKey[M Resource](key ResourceContextKey) ControllerOption[M] {
	return func(c *controller[M]) {
		c.contextKey = key
	}
}

func WithUserAccessFunc[M Resource](accessFunc UserResourceAccessFunc[M]) ControllerOption[M] {
	return func(c *controller[M]) {
		c.userAccessFunc = accessFunc
	}
}
