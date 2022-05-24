package main

import (
	"net/http"

	"github.com/go-chi/chi"

	"github.com/t1mon-ggg/gophermart/internal/pkg/handlers"
)

func main() {
	app := handlers.NewGopherMart()

	r := chi.NewRouter()

	r.Route("/", app.Router)

	http.ListenAndServe(app.Config.Bind, r)
}
