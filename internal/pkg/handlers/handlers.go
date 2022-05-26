package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi"
	chiMiddleware "github.com/go-chi/chi/middleware"
	"github.com/rs/zerolog/log"

	"github.com/t1mon-ggg/gophermart/internal/pkg/config"
	"github.com/t1mon-ggg/gophermart/internal/pkg/helpers"
	mymiddleware "github.com/t1mon-ggg/gophermart/internal/pkg/middleware"
	"github.com/t1mon-ggg/gophermart/internal/pkg/models"
	"github.com/t1mon-ggg/gophermart/internal/pkg/storage"
)

type Gophermart struct {
	Config *config.Config
	db     *storage.Database
}

type data struct {
	user   models.User
	orders []models.Order
}

func NewGopherMart() *Gophermart {
	app := Gophermart{}
	app.Config = config.New()
	s, err := storage.New(app.Config.DBPath)
	if err != nil {
		log.Error().Err(err).Msg("")
		log.Fatal().Msg("Application critical error. Quiting")
		os.Exit(1)
	}
	app.db = s
	return &app
}

func (s *Gophermart) Router(r chi.Router) {
	r.Use(chiMiddleware.Compress(5))
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(mymiddleware.TimeTracer)
	r.Use(mymiddleware.DecompressRequest)
	r.Use(s.AuthChecker)

	r.Post("/api/user/register", s.postRegister)                //All users
	r.Post("/api/user/login", s.postLogin)                      //All users
	r.Post("/api/user/orders", s.postOrders)                    //Authorized only
	r.Get("/api/user/orders", s.getOrders)                      //Authorized only
	r.Get("/api/user/balance", s.getBalance)                    //Authorized only
	r.Post("/api/user/balance/withdraw", s.postBalanceWithdraw) //Authorized only
	r.Get("/api/user/balance/withdraw", s.getBalanceWithdraw)   //Authorized only
	r.MethodNotAllowed(otherHandler)                            //All users
	r.NotFound(otherHandler)                                    //All users
}

func otherHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug().Msg("Wrong request recieved")
	http.Error(w, "Wrong request format", http.StatusBadRequest)
}

func (s *Gophermart) postRegister(w http.ResponseWriter, r *http.Request) {
	var newuser models.User
	ctype := r.Header.Get("Content-Type")
	if ctype != "application/json" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Request body read error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	log.Debug().Msgf("Recieved body %s", string(body))
	err = json.Unmarshal(body, &newuser)
	if err != nil {
		log.Error().Err(err).Msg("Error while parsing JSON body")
		http.Error(w, "Incorrect request format", http.StatusBadRequest)
		return
	}
	if newuser.Name == "" || newuser.Password == "" {
		log.Error().Err(err).Msg("Wrong user data")
		http.Error(w, "Incorrect request format", http.StatusBadRequest)
		return
	}
	pass, err := helpers.SecurePassword(newuser.Password)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	iv := helpers.RandStringRunes(12)
	err = s.db.CreateUser(newuser.Name, pass, iv)
	if err != nil {
		if helpers.UserConflict(err) {
			http.Error(w, "User already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	helpers.SetCookie(w, "username", newuser.Name)
	helpers.SetCookie(w, "user_id", helpers.GenerateCookieValue(newuser.Name, pass, r.RemoteAddr, iv))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte{})
}

func (s *Gophermart) postLogin(w http.ResponseWriter, r *http.Request) {
	var user models.User
	ctype := r.Header.Get("Content-Type")
	if ctype != "application/json" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Request body read error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	log.Debug().Msgf("Recieved body %s", string(body))
	err = json.Unmarshal(body, &user)
	if err != nil {
		log.Error().Err(err).Msg("Error while parsing JSON body")
		http.Error(w, "Incorrect request format", http.StatusBadRequest)
		return
	}
	u, err := s.db.GetUser(user.Name)
	if err != nil {
		log.Error().Err(err).Msg("")
		if helpers.UserNotFound(err) {
			log.Debug().Msgf("User %s not found", user.Name)
			http.Error(w, "Wrond username or password", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !helpers.ComparePassword(user.Password, u.Password) {
		log.Debug().Msgf("User %s not found", user.Name)
		http.Error(w, "Wrond username or password", http.StatusUnauthorized)
		return
	}
	helpers.SetCookie(w, "username", user.Name)
	helpers.SetCookie(w, "user_id", helpers.GenerateCookieValue(user.Name, u.Password, r.RemoteAddr, u.Random))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte{})
}

func (s *Gophermart) postOrders(w http.ResponseWriter, r *http.Request) {
	ctype := r.Header.Get("Content-Type")
	if ctype != "text/plain" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Request body read error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	user, err := helpers.GetUser(r)
	if err != nil {
		log.Debug().Msg("Username cookie missing")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	log.Debug().Msgf("Recieved body %s", string(body))
	order, err := strconv.Atoi(string(body))
	if err != nil {
		log.Error().Err(err).Msg("Incorrect number format")
		http.Error(w, "Incorrect order format", http.StatusUnprocessableEntity)
		return
	}
	if !helpers.CheckOrder(body) {
		http.Error(w, "Incorrect order format", http.StatusUnprocessableEntity)
		return
	}
	log.Debug().Msgf("New order %v from user %v", order, user)
	err = s.db.CreateOrder(order, user)
	if err != nil {
		if helpers.OrderUnique(err) {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte("Order already created by another user"))
			return
		}
		if helpers.OrderExists(err) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Order already uploaded"))
			return
		}
		log.Error().Err(err).Msg("Create order error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Order accepted"))
}

func (s *Gophermart) getOrders(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Application in development", http.StatusInternalServerError)
}

func (s *Gophermart) getBalance(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Application in development", http.StatusInternalServerError)
}

func (s *Gophermart) postBalanceWithdraw(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Application in development", http.StatusInternalServerError)
}

func (s *Gophermart) getBalanceWithdraw(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Application in development", http.StatusInternalServerError)
}

//AuthChecker - check auth cookie for custom urls
func (s *Gophermart) AuthChecker(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var value string
		var user string
		free := false
		if r.RequestURI == "/api/user/register" || r.RequestURI == "/api/user/login" || r.RequestURI == "/" {
			log.Debug().Msg("Skip auth check. All users area")
			free = true
			next.ServeHTTP(w, r)
		}
		if !free {
			cookies := r.Cookies()
			if len(cookies) == 0 {
				log.Debug().Msg("No cookies in request")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			foundC := false
			foundU := false
			for _, cookie := range cookies {
				if cookie.Name == "user_id" {
					foundC = true
					log.Debug().Msg("Cookie 'user_id' was found")
					value = cookie.Value
				}
				if cookie.Name == "username" {
					foundU = true
					log.Debug().Msg("Cookie 'username' was found")
					user = cookie.Value
				}
			}
			if !foundU || !foundC {
				log.Debug().Msg("Cookies 'username' or 'user_id' was not found")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			u, err := s.db.GetUser(user)
			if err != nil {
				log.Error().Err(err).Msg("Get user info error")
				if helpers.UserNotFound(err) {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			ip := r.RemoteAddr
			if !helpers.CompareCookie(value, user, u.Password, ip, u.Random) {
				log.Debug().Msg("Cookie missmatch")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			log.Debug().Msg("Auth cookie processing successful")
			next.ServeHTTP(w, r)
		}
	})
}
