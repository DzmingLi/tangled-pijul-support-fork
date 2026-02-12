package discussions

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"tangled.org/core/appview/middleware"
)

func (d *Discussions) Router(mw *middleware.Middleware) http.Handler {
	r := chi.NewRouter()

	r.Route("/", func(r chi.Router) {
		r.With(middleware.Paginate).Get("/", d.RepoDiscussionsList)

		// Authenticated routes for creating discussions
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(d.oauth))
			r.Get("/new", d.NewDiscussion)
			r.Post("/new", d.NewDiscussion)
		})

		// Single discussion routes
		r.Route("/{discussion}", func(r chi.Router) {
			r.Use(mw.ResolveDiscussion)
			r.Get("/", d.RepoSingleDiscussion)

			// Authenticated routes
			r.Group(func(r chi.Router) {
				r.Use(middleware.AuthMiddleware(d.oauth))

				// Comments
				r.Post("/comment", d.NewComment)

				// Patches - anyone authenticated can add patches
				r.Post("/patches", d.AddPatch)

				// Patch management
				r.Route("/patches/{patchId}", func(r chi.Router) {
					r.Delete("/", d.RemovePatch)
					r.Post("/readd", d.ReaddPatch)
				})

				// Discussion state changes
				r.Post("/close", d.CloseDiscussion)
				r.Post("/reopen", d.ReopenDiscussion)
				r.Post("/merge", d.MergeDiscussion)
			})
		})
	})

	return r
}
