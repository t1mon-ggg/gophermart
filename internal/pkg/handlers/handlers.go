package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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

//sublog - package handlers sub logger
var sublog = log.With().Str("component", "handlers").Logger()

//NewGopherMart - creating new application main work object
func NewGopherMart() *Gophermart {
	app := Gophermart{}
	app.Config = config.New()
	s, err := storage.New(app.Config.DBPath)
	if err != nil {
		sublog.Error().Err(err).Msg("")
		sublog.Fatal().Msg("Application critical error. Quiting")
		os.Exit(1)
	}
	app.db = s
	return &app
}

//Router - creating working router
func (s *Gophermart) Router(r chi.Router) {
	r.Use(chiMiddleware.Compress(5))
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(mymiddleware.TimeTracer)
	r.Use(mymiddleware.DecompressRequest)
	r.Use(s.authChecker)

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

//postRegister - handling/api/user/register on method POST
func (s *Gophermart) postRegister(w http.ResponseWriter, r *http.Request) {
	sublog.Info().Msg("Processing new order")
	var newuser models.User
	ctype := r.Header.Get("Content-Type")
	if ctype != "application/json" {
		sublog.Info().Msg("Content type invalid")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sublog.Error().Err(err).Msg("Request body read error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Debug().Msgf("Recieved body %s", string(body))
	err = json.Unmarshal(body, &newuser)
	if err != nil {
		sublog.Error().Err(err).Msg("Error while parsing JSON body")
		http.Error(w, "Incorrect request format", http.StatusBadRequest)
		return
	}
	sublog.Debug().Msgf("Parsed from json. Name: %v, Password: %v", newuser.Name, newuser.Password)
	if newuser.Name == "" || newuser.Password == "" {
		sublog.Error().Err(err).Msg("Wrong user data")
		http.Error(w, "Incorrect request format", http.StatusBadRequest)
		return
	}
	pass, err := helpers.SecurePassword(newuser.Password)
	if err != nil {
		sublog.Error().Err(err).Msg("")
	}
	iv := helpers.RandStringRunes(12)
	err = s.db.CreateUser(newuser.Name, pass, iv)
	if err != nil {
		if helpers.UserConflict(err) {
			sublog.Info().Msgf("User %v already exist", newuser.Name)
			http.Error(w, "User already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	helpers.SetCookie(w, "username", newuser.Name)
	helpers.SetCookie(w, "user_id", helpers.GenerateCookieValue(newuser.Name, pass, r.RemoteAddr, iv))
	sublog.Info().Msgf("User %v registered", newuser.Name)
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte{})
	if err != nil {
		log.Debug().Err(err).Msg("Error in http.ResponseWriter")
	}
}

//postLogin - handling/api/user/login on method POST
func (s *Gophermart) postLogin(w http.ResponseWriter, r *http.Request) {
	sublog.Info().Msg("Processing authorization request")
	var user models.User
	ctype := r.Header.Get("Content-Type")
	if ctype != "application/json" {
		sublog.Info().Msg("Invalid content type")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sublog.Error().Err(err).Msg("Request body read error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Debug().Msgf("Recieved body %s", string(body))
	err = json.Unmarshal(body, &user)
	if err != nil {
		sublog.Error().Err(err).Msg("Error while parsing JSON body")
		http.Error(w, "Incorrect request format", http.StatusBadRequest)
		return
	}
	sublog.Debug().Msgf("Parsed from json. Login: %v, Password: %v", user.Name, user.Password)
	u, err := s.db.GetUser(user.Name)
	if err != nil {
		if helpers.EmptyRow(err) {
			sublog.Info().Msgf("User %s not found", user.Name)
			http.Error(w, "Wrond username or password", http.StatusUnauthorized)
			return
		}
		sublog.Error().Err(err).Msg("")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !helpers.ComparePassword(user.Password, u.Password) {
		sublog.Info().Msgf("Password %v invalid", user.Password)
		http.Error(w, "Wrond username or password", http.StatusUnauthorized)
		return
	}
	helpers.SetCookie(w, "username", user.Name)
	helpers.SetCookie(w, "user_id", helpers.GenerateCookieValue(user.Name, u.Password, r.RemoteAddr, u.Random))
	sublog.Info().Msgf("User %v authorized", user.Name)
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte{})
	if err != nil {
		log.Debug().Err(err).Msg("Error in http.ResponseWriter")
	}
}

//postOrders - handling/api/user/orders on method POST
func (s *Gophermart) postOrders(w http.ResponseWriter, r *http.Request) {
	sublog.Info().Msg("Processing new order")
	ctype := r.Header.Get("Content-Type")
	if !strings.Contains(ctype, "text/plain") {
		sublog.Info().Msg("Invalid content type")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sublog.Error().Err(err).Msg("Request body read error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	user, err := helpers.GetUser(r)
	if err != nil {
		sublog.Info().Msg("Username cookies missing or invalid")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	sublog.Debug().Msgf("Recieved body %s", string(body))
	order := string(body)
	if !helpers.CheckOrder(body) {
		sublog.Info().Msg("Invalid order number")
		http.Error(w, "Incorrect order format", http.StatusUnprocessableEntity)
		return
	}
	sublog.Debug().Msgf("New order %v from user %v", order, user)
	err = s.db.CreateOrder(order, user)
	if err != nil {
		if helpers.OrderUnique(err) {
			sublog.Info().Msg("Order already exist. Created by another user")
			w.WriteHeader(http.StatusConflict)
			i, err := w.Write([]byte("Order already created by another user"))
			log.Debug().Msgf("%v bytes wrote to ResponseWriter", i)
			if err != nil {
				log.Debug().Err(err).Msg("Error in http.ResponseWriter")
			}
			return
		}
		if helpers.OrderExists(err) {
			sublog.Info().Msg("Order already processed early")
			w.WriteHeader(http.StatusOK)
			i, err := w.Write([]byte("Order already uploaded"))
			log.Debug().Msgf("%v bytes wrote to ResponseWriter", i)
			if err != nil {
				log.Debug().Err(err).Msg("Error in http.ResponseWriter")
			}
			return
		}
		sublog.Error().Err(err).Msg("Create order error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Info().Msg("Order successfully created")
	w.WriteHeader(http.StatusAccepted)
	i, err := w.Write([]byte("Order accepted"))
	log.Debug().Msgf("%v bytes wrote to ResponseWriter", i)
	if err != nil {
		log.Debug().Err(err).Msg("Error in http.ResponseWriter")
	}
	go s.accrualAPI(user, order)
}

//getBalance - handling/api/user/orders on method GET
func (s *Gophermart) getOrders(w http.ResponseWriter, r *http.Request) {
	user, err := helpers.GetUser(r)
	sublog.Info().Msgf("Request user's %v orders", user)
	if err != nil {
		sublog.Info().Msg("Username not recognized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	o, err := s.db.GetOrders(user)
	if err != nil {
		sublog.Error().Err(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Debug().Msgf("Get_Orders result is %v", o)
	if len(o) == 0 {
		sublog.Debug().Msg("Orders not found")
		http.Error(w, "No orders found", http.StatusNoContent)
		return
	}
	body, err := json.Marshal(o)
	if err != nil {
		sublog.Error().Err(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Debug().Msg("Request list of orders complete")
	w.Header().Add("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	i, err := w.Write(body)
	log.Debug().Msgf("%v bytes wrote to ResponseWriter", i)
	if err != nil {
		log.Debug().Err(err).Msg("Error in http.ResponseWriter")
	}

}

//getBalance - handling/api/user/balance on method GET
func (s *Gophermart) getBalance(w http.ResponseWriter, r *http.Request) {
	sublog.Info().Msg("Processing request of a balance")
	user, err := helpers.GetUser(r)
	if err != nil {
		sublog.Info().Msg("Username not recognized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	sublog.Debug().Msgf("Get_Balance user is %v", user)
	balance, err := s.db.GetBalance(user)
	if err != nil {
		sublog.Error().Err(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Debug().Msgf("User's %v balance is %v and withdraw is %v", user, balance.Balance, balance.Withdraws)
	body, err := json.Marshal(balance)
	if err != nil {
		sublog.Error().Err(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Debug().Msgf("Balance JSON: %v", string(body))
	sublog.Info().Msg("Request of user's balance complete")
	w.Header().Add("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	i, err := w.Write(body)
	log.Debug().Msgf("%v bytes wrote to ResponseWriter", i)
	if err != nil {
		log.Debug().Err(err).Msg("Error in http.ResponseWriter")
	}
}

//postBalanceWithdraw - handling/api/user/balance/withdraw on method POST
func (s *Gophermart) postBalanceWithdraw(w http.ResponseWriter, r *http.Request) {
	type withdrawn struct {
		Number string  `json:"order"`
		Sum    float32 `json:"sum"`
	}
	sublog.Info().Msg("Processing request of a new withdrawn")
	ctype := r.Header.Get("Content-Type")
	if ctype != "application/json" {
		sublog.Info().Msg("Invalid content type")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	user, err := helpers.GetUser(r)
	if err != nil {
		sublog.Info().Msg("Username cookies missing or invalid")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	a := withdrawn{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sublog.Error().Err(err).Msg("Request body read error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Debug().Msgf("Recieved body: %v", string(body))
	err = json.Unmarshal(body, &a)
	if err != nil {
		sublog.Error().Err(err).Msg("Error while parsing JSON body")
		http.Error(w, "Incorrect request format", http.StatusBadRequest)
		return
	}
	sublog.Debug().Msgf("Parsed order %v and sum %v", a.Number, a.Sum)
	if !helpers.CheckOrder([]byte(a.Number)) {
		sublog.Info().Msg("Wrong order number")
		http.Error(w, "Invalid order number", http.StatusUnprocessableEntity)
		return
	}
	err = s.db.CreateWithdraw(a.Sum, user, a.Number)
	if err != nil {
		if helpers.BalanceTooLow(err) {
			sublog.Info().Msg("Not enough bonuses on the balance")
			http.Error(w, "There are not enough funds in the account", http.StatusPaymentRequired)
			return
		}
	}
	sublog.Info().Msg("Withdrwan successfulyy processed")
	w.WriteHeader(http.StatusOK)
	i, err := w.Write(body)
	log.Debug().Msgf("%v bytes wrote to ResponseWriter", i)
	if err != nil {
		log.Debug().Err(err).Msg("Error in http.ResponseWriter")
	}
}

//getBalanceWithdraw - handling/api/user/balance/withdraw on method GET
func (s *Gophermart) getBalanceWithdraw(w http.ResponseWriter, r *http.Request) {
	sublog.Info().Msg("Processing request of a withdraws")
	user, err := helpers.GetUser(r)
	if err != nil {
		sublog.Info().Msg("Username not recognized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	sublog.Debug().Msgf("Get_Withdraws for %v", user)
	withdraws, err := s.db.GetWithdraws(user)
	if err != nil {
		if helpers.EmptyRow(err) {
			sublog.Info().Msg("Withdraws not found")
			http.Error(w, "Withdraws not found", http.StatusNoContent)
			return
		}
		sublog.Info().Msg("Error in requsting withdraws")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if len(withdraws) == 0 {
		sublog.Info().Msg("Withdraws not found")
		http.Error(w, "Withdraws not found", http.StatusNoContent)
		return
	}
	log.Debug().Msgf("Withdraws for %v: %v", user, withdraws)
	body, err := json.Marshal(withdraws)
	if err != nil {
		sublog.Error().Err(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Info().Msg("Request of user's withdraws complete")
	w.Header().Add("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	i, err := w.Write(body)
	log.Debug().Msgf("%v bytes wrote to ResponseWriter", i)
	if err != nil {
		log.Debug().Err(err).Msg("Error in http.ResponseWriter")
	}

}

//getBalanceWithdraw - handling other paths and methods
func otherHandler(w http.ResponseWriter, r *http.Request) {
	sublog.Info().Msg("Wrong request recieved")
	http.Error(w, "Wrong request format", http.StatusBadRequest)
}

//AuthChecker - checking authorization cookies in requests
func (s *Gophermart) authChecker(next http.Handler) http.Handler {
	sublog.Debug().Msg("Request authorization tokens check")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var value string
		var user string
		free := false
		if r.RequestURI == "/api/user/register" || r.RequestURI == "/api/user/login" || r.RequestURI == "/" {
			sublog.Debug().Msg("Skip auth check. All users area")
			free = true
			next.ServeHTTP(w, r)
		}
		if !free {
			cookies := r.Cookies()
			if len(cookies) == 0 {
				sublog.Debug().Msg("No cookies in request")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			foundC := false
			foundU := false
			for _, cookie := range cookies {
				if cookie.Name == "user_id" {
					foundC = true
					sublog.Debug().Msg("Cookie 'user_id' was found")
					value = cookie.Value
				}
				if cookie.Name == "username" {
					foundU = true
					sublog.Debug().Msg("Cookie 'username' was found")
					user = cookie.Value
				}
			}
			if !foundU || !foundC {
				sublog.Debug().Msg("Cookies 'username' or 'user_id' was not found")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			u, err := s.db.GetUser(user)
			if err != nil {
				sublog.Error().Err(err).Msg("Get user info error")
				if helpers.EmptyRow(err) {
					sublog.Debug().Msg("User not found")
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			ip := r.RemoteAddr
			if !helpers.CompareCookie(value, user, u.Password, ip, u.Random) {
				sublog.Debug().Msg("Cookie is not valid")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			sublog.Debug().Msg("Authorization cookie processing end")
			next.ServeHTTP(w, r)
		}
	})
}

func (s *Gophermart) accrualAPI(login, order string) {
	subsublog := sublog.With().Str("subcomponent", "accrual api").Logger()
	subsublog.Info().Msg("Processing new order withh accrual service.")
	acc := models.Accrual{}
	url := fmt.Sprintf("%s/api/orders/%s", s.Config.AccSystem, order)
	subsublog.Debug().Msgf("Actual accrual system request is '%s'", url)
	client := http.Client{}
	request, err := http.NewRequest(http.MethodGet, url, bytes.NewReader([]byte{}))
	if err != nil {
		subsublog.Error().Err(err).Msg("Error in creating request to accrual system")
		return
	}
	var wait bool
	for !wait {
		response, err := client.Do(request)
		if err != nil {
			subsublog.Error().Err(err).Msg("Error in request to accrual system")
			return
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			if response.StatusCode == http.StatusTooManyRequests {
				retryTime := response.Header.Get("Retry-After")
				t, err := strconv.Atoi(retryTime)
				if err != nil {
					log.Debug().Err(err).Msg("String to int convertation failed")
				}
				time.Sleep(time.Duration(t) * time.Second)
			} else {
				subsublog.Debug().Msgf("Status code not 200. Recieved code %d. Waiting for 1 second to the next try", response.StatusCode)
				time.Sleep(1 * time.Second)
				continue
			}

		}
		body, err := io.ReadAll(response.Body)
		if err != nil {
			subsublog.Error().Err(err)
		}
		subsublog.Debug().Msgf("Recieved json is %v", string(body))
		err = json.Unmarshal(body, &acc)
		if err != nil {
			subsublog.Error().Err(err).Msg("Error in unmarshaling answer from accrual system")
		}
		sublog.Debug().Msgf("Parsed from json. Order: %v, Status: %v, Accrual: %v", acc.Order, acc.Status, acc.Value)
		if acc.Status == "INVALID" || acc.Status == "PROCESSED" {
			subsublog.Debug().Msg("Accrual calculation in progress.")
			wait = true
		}
		if !wait {
			sublog.Debug().Msg("Waiting for 1 seeconds to the next try")
			time.Sleep(1 * time.Second)
		}
	}
	if acc.Status == "INVALID" {
		acc.Value = 0
	}
	subsublog.Info().Msgf("Accrual processing complete. Processing status is %v", acc.Status)
	err = s.db.UpdateOrder(order, acc.Status, acc.Value)
	if err != nil {
		subsublog.Error().Err(err)
		return
	}
	err = s.db.UpdateBalance(login, acc.Value)
	if err != nil {
		subsublog.Error().Err(err)
		return
	}
	subsublog.Info().Msg("Accrual processing complete. Exit from goroutine")
}
