package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

type data struct {
	user   models.User
	orders []models.Order
}

var sublog = log.With().Str("component", "handlers").Logger()

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
	sublog.Info().Msg("Wrong request recieved")
	http.Error(w, "Wrong request format", http.StatusBadRequest)
}

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
	w.Write([]byte{})
}

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
	w.Write([]byte{})
}

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
			w.Write([]byte("Order already created by another user"))
			return
		}
		if helpers.OrderExists(err) {
			sublog.Info().Msg("Order already processed early")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Order already uploaded"))
			return
		}
		sublog.Error().Err(err).Msg("Create order error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Info().Msg("Order successfully created")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Order accepted"))
	go s.AccrualAPI(user, order)
}

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
	w.Write(body)
}

func (s *Gophermart) getBalance(w http.ResponseWriter, r *http.Request) {
	sublog.Info().Msg("Processing request of a balance")
	type b struct {
		Balance   float32 `json:"balance"`
		Withdrawn float32 `json:"withdrawn"`
	}
	answer := b{}
	user, err := helpers.GetUser(r)
	if err != nil {
		sublog.Info().Msg("Username not recognized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	sublog.Debug().Msgf("Get_Balance user is %v", user)
	balance, withdrawn, err := s.db.GetBalance(user)
	if err != nil {
		sublog.Error().Err(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Debug().Msgf("User's %v balance is %v and withdrawn is %v", user, balance, withdrawn)
	if balance == 0 && withdrawn == 0 {
		sublog.Info().Msg("Transactions not found")
		http.Error(w, "Transactions not found", http.StatusNoContent)
		return
	}
	answer.Balance = balance
	answer.Withdrawn = withdrawn
	body, err := json.Marshal(answer)
	if err != nil {
		sublog.Error().Err(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Debug().Msgf("Balance JSON: %v", string(body))
	sublog.Info().Msg("Request of user's balance complete")
	w.Header().Add("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func (s *Gophermart) postBalanceWithdraw(w http.ResponseWriter, r *http.Request) {
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
	withdrawn := models.Order{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sublog.Error().Err(err).Msg("Request body read error")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(body, &withdrawn)
	if err != nil {
		sublog.Error().Err(err).Msg("Error while parsing JSON body")
		http.Error(w, "Incorrect request format", http.StatusBadRequest)
		return
	}
	err = s.db.UpdateWithdrawn(withdrawn.Withdrawn, user, withdrawn.Number)
	if err != nil {
		if helpers.BalanceTooLow(err) {
			sublog.Info().Msg("Not enough bonuses on the balance")
			http.Error(w, "There are not enough funds in the account", http.StatusPaymentRequired)
			return
		}
		if helpers.WithdrawnError(err) {
			sublog.Info().Msg("Wrong order number")
			http.Error(w, "Invalid order number", http.StatusUnprocessableEntity)
			return
		}
	}
	sublog.Info().Msg("Withdrwan successfulyy processed")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func (s *Gophermart) getBalanceWithdraw(w http.ResponseWriter, r *http.Request) {
	sublog.Info().Msg("Processing request of a withdrawns")
	user, err := helpers.GetUser(r)
	if err != nil {
		sublog.Info().Msg("Username not recognized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	sublog.Debug().Msgf("Get_Withdrawns for %v", user)
	withdrawns, err := s.db.GetWithdrawns(user)
	if err != nil {
		if helpers.EmptyRow(err) {
			sublog.Info().Msg("Withdrawns not found")
			http.Error(w, "Withdrawns not found", http.StatusNoContent)
			return
		}
		sublog.Info().Msg("Error in requsting withdrawns")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if len(withdrawns) == 0 {
		sublog.Info().Msg("Withdrawns not found")
		http.Error(w, "Withdrawns not found", http.StatusNoContent)
		return
	}
	body, err := json.Marshal(withdrawns)
	if err != nil {
		sublog.Error().Err(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sublog.Info().Msg("Request of user's withdrawns complete")
	w.Header().Add("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)

}

//AuthChecker - check auth cookie for custom urls
func (s *Gophermart) AuthChecker(next http.Handler) http.Handler {
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

func (s *Gophermart) AccrualAPI(login, order string) {
	subsublog := sublog.With().Str("component", "accrual api").Logger()
	subsublog.Info().Msg("Processing new order withh accrual service.")
	/*
		Accrual answer example

			200 OK HTTP/1.1
		  Content-Type: application/json
		  ...

		  {
		      "order": "<number>",
		      "status": "PROCESSED",
		      "accrual": 500
		  }
	*/
	type accrual struct {
		Order  string  `json:"order"`             //Order number.
		Status string  `json:"status"`            //Order status. Allowed values are "REGISTERED", "INVALID", "PROCESSING", "PROCESSED". Status "INVALID" or "PROCESSED" are final.
		Val    float32 `json:"accrual,omitempty"` //Calculated accrual value.
	}
	var acc accrual
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
			subsublog.Debug().Msgf("Status code not 200. Recieved code %d. Waiting for 15 seconds to the next try", response.StatusCode)
			time.Sleep(15 * time.Second)
			continue
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
		if acc.Status == "INVALID" || acc.Status == "PROCESSED" {
			subsublog.Debug().Msg("Accrual calculation in progress. Waiting for 15 seeconds to the next try")
			wait = true
		}
		if !wait {
			time.Sleep(15 * time.Second)
		}
	}
	if acc.Status == "INVALID" {

		acc.Val = 0
	}
	subsublog.Info().Msgf("Accrual processing complete. Processing status is %v", acc.Status)
	err = s.db.UpdateOrder(order, acc.Status, acc.Val)
	if err != nil {
		subsublog.Error().Err(err)
		return
	}
	err = s.db.UpdateBalance(login, acc.Val)
	if err != nil {
		subsublog.Error().Err(err)
		return
	}
	subsublog.Info().Msg("Accrual processing complete. Exit from goroutine")
}
