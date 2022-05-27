package main

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/rs/zerolog"

	"github.com/t1mon-ggg/gophermart/internal/pkg/handlers"
)

func main() {
	app := handlers.NewGopherMart()
	//remove in prod
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	//

	r := chi.NewRouter()

	r.Route("/", app.Router)

	http.ListenAndServe(app.Config.Bind, r)
}
