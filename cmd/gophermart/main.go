package main

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/rs/zerolog"

	"github.com/t1mon-ggg/gophermart/internal/pkg/handlers"
)

func main() {
	//remove in prod
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	//
	app := handlers.NewGopherMart()

	r := chi.NewRouter()

	r.Route("/", app.Router)

	http.ListenAndServe(app.Config.Bind, r)
}
