package handles

import (
	"net/http"

	"github.com/go-chi/chi"
	chiMiddleware "github.com/go-chi/chi/middleware"
)

type mart struct {
	config struct{}
	data   struct{}
}

func (s *mart) Router(r chi.Router) {
	r.Post("/api/user/register", s.postRegister)
	r.Post("/api/user/login", s.postLogin)
	r.Post("/api/user/orders", s.postOrders)
	r.Get("/api/user/orders", s.getOrders)
	r.Get("/api/user/balance", s.getBalance)
	r.Post("/api/user/balance/withdraw", s.postBalanceWithdraw)
	r.Get("/api/user/balance/withdraw", s.getBalanceWithdraw)
}

func (s *mart) postRegister(w http.ResponseWriter, r *http.Request) {

}

func (s *mart) postLogin(w http.ResponseWriter, r *http.Request) {

}

func (s *mart) postOrders(w http.ResponseWriter, r *http.Request) {

}

func (s *mart) getOrders(w http.ResponseWriter, r *http.Request) {

}

func (s *mart) getBalance(w http.ResponseWriter, r *http.Request) {

}

func (s *mart) postBalanceWithdraw(w http.ResponseWriter, r *http.Request) {

}

func (s *mart) getBalanceWithdraw(w http.ResponseWriter, r *http.Request) {

}

func (s *mart) Middlewares(r *chi.Mux) {
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
}
