package internal

import (
	"github.com/go-chi/render"
)

type Resource interface {
	Model
	ToDTO() render.Renderer
}
