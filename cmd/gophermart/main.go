package main

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/rs/zerolog/log"

	"github.com/t1mon-ggg/gophermart/internal/pkg/handlers"
)

func main() {
	app := handlers.NewGopherMart()
	log.Info().Msg("New app struct created")

	r := chi.NewRouter()
	log.Info().Msg("Chi reouter created")

	r.Route("/", app.Router)
	log.Info().Msg("Chi router configured. Starting web bind")

	http.ListenAndServe(app.Config.Bind, r)
}
