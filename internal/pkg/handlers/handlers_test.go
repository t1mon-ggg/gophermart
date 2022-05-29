package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t1mon-ggg/gophermart/internal/pkg/config"
	"github.com/t1mon-ggg/gophermart/internal/pkg/models"
	"github.com/t1mon-ggg/gophermart/internal/pkg/storage"
)

func newServer(t *testing.T) (*cookiejar.Jar, *chi.Mux, *Gophermart) {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	mart := Gophermart{
		Config: &config.Config{
			Bind:      "",
			DBPath:    "postgresql://postgres:admin@127.0.0.1:5432/gophermart?sslmode=disable",
			AccSystem: "http://127.0.0.1:8080",
		},
	}
	s, err := storage.New(mart.Config.DBPath)
	require.NoError(t, err)
	mart.db = s
	r := chi.NewRouter()
	r.Route("/", mart.Router)
	return jar, r, &mart
}

func gziped(ctype map[string]string) bool {
	for i := range ctype {
		if i == "Content-Encoding" && ctype[i] == "gzip" {
			return true
		}
	}
	return false
}

func compress(data []byte) ([]byte, error) {
	log.Debug().Msg("Compressing...")
	log.Debug().Msgf("Not compressed body is %v", string(data))
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(data)); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func decompress(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	unzipped, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}
	return unzipped, nil
}

func testRequest(t *testing.T, ts *httptest.Server, jar *cookiejar.Jar, method, path, body string, ctype map[string]string) (*http.Response, string) {

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Jar: jar,
	}
	var bodyreq *strings.Reader
	if gziped(ctype) {
		c, _ := compress([]byte(body))
		bodyreq = strings.NewReader(string(c))
	} else {
		bodyreq = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, ts.URL+path, bodyreq)
	require.NoError(t, err)
	for i := range ctype {
		req.Header.Set(i, ctype[i])
	}

	resp, err := client.Do(req)
	require.NoError(t, err)

	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		log.Debug().Msg("Compressed response found")
		respBody, err = decompress(respBody)
		require.NoError(t, err)
	}
	defer resp.Body.Close()
	return resp, string(respBody)
}

func userReq(t *testing.T, val interface{}) string {
	d, err := json.Marshal(val)
	require.NoError(t, err)
	return string(d)
}
func Test_otherHandler(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	jar, r, _ := newServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()
	t.Run("Test other method handler", func(t *testing.T) {
		ctype := map[string]string{
			"Content-Type": "text/plain; charset=utf-8",
		}
		response, _ := testRequest(t, ts, jar, http.MethodPut, "/", "", ctype)
		defer response.Body.Close()
		require.Equal(t, http.StatusBadRequest, response.StatusCode)

		response, _ = testRequest(t, ts, jar, http.MethodPut, "/api/user/register", "", ctype)
		defer response.Body.Close()
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	})
}

func TestGophermart_postRegister(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	jar, r, s := newServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()
	type args struct {
		name     string
		password string
	}
	tests := []struct {
		name  string
		ctype map[string]string
		args  args
		want  int
	}{
		{
			name: "Register new user",
			ctype: map[string]string{
				"Content-Type": "application/json",
			},
			args: args{
				name:     "user111",
				password: "password111",
			},
			want: http.StatusOK,
		},
		{
			name: "Duplicated user",
			ctype: map[string]string{
				"Content-Type": "application/json",
			},
			args: args{
				name:     "user111",
				password: "password111",
			},
			want: http.StatusConflict,
		},
		{
			name: "Wrong content-type",
			ctype: map[string]string{
				"Content-Type": "text/plain; charset=utf-8",
			},
			args: args{
				name:     "user111",
				password: "password111",
			},
			want: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := userReq(t, tt.args)
			response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, tt.ctype)
			defer response.Body.Close()
			assert.Equal(t, tt.want, response.StatusCode)
		})
	}
	err := s.db.DeleteContent("orders")
	require.NoError(t, err)
	err = s.db.DeleteContent("balance")
	require.NoError(t, err)
	err = s.db.DeleteContent("withdraws")
	require.NoError(t, err)
	err = s.db.DeleteContent("users")
	require.NoError(t, err)
}

func TestGophermart_postLogin(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	jar, r, s := newServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()
	type args struct {
		name     string
		password string
	}
	tests := []struct {
		name  string
		ctype map[string]string
		args  args
		want  int
	}{
		{
			name: "Login user",
			ctype: map[string]string{
				"Content-Type": "application/json",
			},
			args: args{
				name:     "user111",
				password: "password111",
			},
			want: http.StatusOK,
		},
		{
			name: "Wrong content-type",
			ctype: map[string]string{
				"Content-Type": "text/plain; charset=utf-8",
			},
			args: args{
				name:     "user111",
				password: "password111",
			},
			want: http.StatusBadRequest,
		},
		{
			name: "Wrong user",
			ctype: map[string]string{
				"Content-Type": "application/json",
			},
			args: args{
				name:     "user112",
				password: "password111",
			},
			want: http.StatusUnauthorized,
		},
		{
			name: "Wrong password",
			ctype: map[string]string{
				"Content-Type": "application/json",
			},
			args: args{
				name:     "user111",
				password: "password112",
			},
			want: http.StatusUnauthorized,
		},
	}
	user := models.User{
		Name:     "user111",
		Password: "password111",
	}
	body := userReq(t, user)
	response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
		"Content-Type": "application/json",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := userReq(t, tt.args)
			response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/login", body, tt.ctype)
			defer response.Body.Close()
			assert.Equal(t, tt.want, response.StatusCode)
		})
	}
	err := s.db.DeleteContent("orders")
	require.NoError(t, err)
	err = s.db.DeleteContent("balance")
	require.NoError(t, err)
	err = s.db.DeleteContent("withdraws")
	require.NoError(t, err)
	err = s.db.DeleteContent("users")
	require.NoError(t, err)
}

func TestGophermart_postOrder(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	jar, r, s := newServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()
	type args struct {
		name  string
		order string
	}
	tests := []struct {
		name  string
		ctype map[string]string
		args  args
		want  int
	}{
		{
			name: "New order",
			ctype: map[string]string{
				"Content-Type": "text/plain",
			},
			args: args{
				name:  "user111",
				order: "123455",
			},
			want: http.StatusAccepted,
		},
		{
			name: "Duplicate order by same user",
			ctype: map[string]string{
				"Content-Type": "text/plain",
			},
			args: args{
				name:  "user111",
				order: "123455",
			},
			want: http.StatusOK,
		},
		{
			name: "Wrong order format",
			ctype: map[string]string{
				"Content-Type": "application/json",
			},
			args: args{
				name:  "user111",
				order: "123455",
			},
			want: http.StatusBadRequest,
		},
		{
			name: "Wrong order format",
			ctype: map[string]string{
				"Content-Type": "text/plain",
			},
			args: args{
				name:  "user111",
				order: "abcd",
			},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "Wrong order format",
			ctype: map[string]string{
				"Content-Type": "text/plain",
			},
			args: args{
				name:  "user111",
				order: "1234.5",
			},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "Duplicate order by another unauthorized user",
			ctype: map[string]string{
				"Content-Type": "text/plain",
			},
			args: args{
				name:  "user112",
				order: "123455",
			},
			want: http.StatusUnauthorized,
		},
		{
			name: "Duplicate order by another authorized user",
			ctype: map[string]string{
				"Content-Type": "text/plain",
			},
			args: args{
				name:  "user112",
				order: "123455",
			},
			want: http.StatusConflict,
		},
	}
	user := models.User{
		Name:     "user111",
		Password: "password111",
	}
	body := userReq(t, user)
	response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
		"Content-Type": "application/json",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Duplicate order by another unauthorized user" {
				jar, _ = cookiejar.New(nil)
			}
			if tt.name == "Duplicate order by another authorized user" {
				user := models.User{
					Name:     "user112",
					Password: "password112",
				}
				body := userReq(t, user)
				response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
					"Content-Type": "application/json",
				})
				defer response.Body.Close()
			}
			response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/orders", tt.args.order, tt.ctype)
			defer response.Body.Close()
			assert.Equal(t, tt.want, response.StatusCode)
		})
	}
	err := s.db.DeleteContent("orders")
	require.NoError(t, err)
	err = s.db.DeleteContent("balance")
	require.NoError(t, err)
	err = s.db.DeleteContent("withdraws")
	require.NoError(t, err)
	err = s.db.DeleteContent("users")
	require.NoError(t, err)
}

func TestGophermart_getOrders(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	jar, r, s := newServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()
	user := models.User{
		Name:     "user111",
		Password: "password111",
	}
	body := userReq(t, user)
	response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
		"Content-Type": "application/json",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	response, _ = testRequest(t, ts, jar, http.MethodPost, "/api/user/orders", "123455", map[string]string{
		"Content-Type": "text/plain",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusAccepted, response.StatusCode)
	tests := []struct {
		name string
		user string
		want []models.Order
		code int
	}{
		{
			name: "Get valid orders",
			user: "user1",
			want: []models.Order{
				{
					Number:  "123455",
					Status:  "PROCESSED",
					AccRual: 0,
				},
			},
			code: http.StatusOK,
		},
		{
			name: "Get unauthorized orders",
			user: "user2",
			want: []models.Order{},
			code: http.StatusUnauthorized,
		},
		{
			name: "Get empty orders",
			user: "user2",
			want: []models.Order{},
			code: http.StatusNoContent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Get unauthorized orders" {
				jar, _ = cookiejar.New(nil)
			}
			if tt.name == "Get empty orders" {
				user := models.User{
					Name:     "user112",
					Password: "password112",
				}
				body := userReq(t, user)
				response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
					"Content-Type": "application/json",
				})
				defer response.Body.Close()
			}
			response, _ := testRequest(t, ts, jar, http.MethodGet, "/api/user/orders", "", map[string]string{})
			defer response.Body.Close()
			assert.Equal(t, tt.code, response.StatusCode)
		})
	}
	err := s.db.DeleteContent("orders")
	require.NoError(t, err)
	err = s.db.DeleteContent("balance")
	require.NoError(t, err)
	err = s.db.DeleteContent("withdraws")
	require.NoError(t, err)
	err = s.db.DeleteContent("users")
	require.NoError(t, err)
}

func testaccrualrequests(t *testing.T, config *config.Config) {
	urlGoods := fmt.Sprintf("%s/%s", config.AccSystem, "api/goods")
	urlOrders := fmt.Sprintf("%s/%s", config.AccSystem, "api/orders")
	bodyGoods := `{ "match": "LG", "reward": 5, "reward_type": "%" }`
	bodyOrders := `{ "order": "123455", "goods": [ { "description": "LG Monitor", "price": 50000.0 } ] }`
	client := http.Client{}
	reqGoods, err := http.NewRequest(http.MethodPost, urlGoods, bytes.NewReader([]byte(bodyGoods)))
	require.NoError(t, err)
	reqGoods.Header.Add("Content-Type", "application/json")
	response, err := client.Do(reqGoods)
	require.NoError(t, err)
	log.Debug().Msg("Goods request completed")
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)

	reqOrders, err := http.NewRequest(http.MethodPost, urlOrders, bytes.NewReader([]byte(bodyOrders)))
	require.NoError(t, err)
	reqOrders.Header.Add("Content-Type", "application/json")
	response, err = client.Do(reqOrders)
	require.NoError(t, err)
	log.Debug().Msg("Orders request completed")
	defer response.Body.Close()
	require.Equal(t, http.StatusAccepted, response.StatusCode)
}

func TestGophermart_AccrualAPI(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	cmd := exec.Command("C:\\Users\\пользователь\\Documents\\Education\\gophermart\\cmd\\accrual\\accrual_windows_amd64.exe")
	err := cmd.Start()
	require.NoError(t, err)
	defer cmd.Process.Kill()
	time.Sleep(10 * time.Second)
	log.Debug().Msgf("Accrual stated with pid %v", cmd.Process.Pid)
	require.NotZero(t, cmd.Process.Pid)
	jar, r, s := newServer(t)
	testaccrualrequests(t, s.Config)
	ts := httptest.NewServer(r)
	defer ts.Close()
	user := models.User{
		Name:     "user111",
		Password: "password111",
	}
	body := userReq(t, user)
	response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
		"Content-Type": "application/json",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	response, _ = testRequest(t, ts, jar, http.MethodPost, "/api/user/orders", "123455", map[string]string{"Content-type": "text/plain"})
	defer response.Body.Close()
	assert.Equal(t, http.StatusAccepted, response.StatusCode)
	tests := []struct {
		name  string
		login string
		order string
	}{
		{
			name:  "Valid order",
			login: "user111",
			order: "123455",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.accrualAPI(tt.login, tt.order)
			orders, _ := s.db.GetOrders(tt.login)
			for _, order := range orders {
				status := false
				if order.Status == "INVALID" || order.Status == "PROCESSED" {
					status = true
				}
				require.True(t, status)
			}
		})
	}
	err = s.db.DeleteContent("orders")
	require.NoError(t, err)
	err = s.db.DeleteContent("balance")
	require.NoError(t, err)
	err = s.db.DeleteContent("withdraws")
	require.NoError(t, err)
	err = s.db.DeleteContent("users")
	require.NoError(t, err)
}

func TestGophermart_getBalance(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	jar, r, s := newServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()
	user := models.User{
		Name:     "user111",
		Password: "password111",
	}
	body := userReq(t, user)
	response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
		"Content-Type": "application/json",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	type wanted struct {
		code    int
		balance models.Balance
		cType   string
	}
	tests := []struct {
		name   string
		want   wanted
		status int
	}{
		{
			name: "valid user",
			want: wanted{
				code: http.StatusOK,
				balance: models.Balance{
					Balance:   0,
					Withdraws: 0,
				},
				cType: "application/json",
			},
			status: http.StatusOK,
		},
		{
			name: "invalid user",
			want: wanted{
				code: http.StatusUnauthorized,
			},
			status: http.StatusUnauthorized,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "invalid user" {
				jar, _ = cookiejar.New(nil)
			}
			response, body := testRequest(t, ts, jar, http.MethodGet, "/api/user/balance", "", nil)
			defer response.Body.Close()
			require.Equal(t, tt.want.code, response.StatusCode)
			if tt.want.code == http.StatusOK {
				require.Equal(t, tt.want.cType, response.Header.Get("Content-type"))
				b := models.Balance{}
				err := json.Unmarshal([]byte(body), &b)
				require.NoError(t, err)
				require.Equal(t, tt.want.balance, b)
			}
		})
		err := s.db.DeleteContent("orders")
		require.NoError(t, err)
		err = s.db.DeleteContent("balance")
		require.NoError(t, err)
		err = s.db.DeleteContent("withdraws")
		require.NoError(t, err)
		err = s.db.DeleteContent("users")
		require.NoError(t, err)
	}
}

func TestGophermart_postBalanceWithdraw(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	cmd := exec.Command("C:\\Users\\пользователь\\Documents\\Education\\gophermart\\cmd\\accrual\\accrual_windows_amd64.exe")
	err := cmd.Start()
	require.NoError(t, err)
	defer cmd.Process.Kill()
	time.Sleep(10 * time.Second)
	log.Debug().Msgf("Accrual stated with pid %v", cmd.Process.Pid)
	require.NotZero(t, cmd.Process.Pid)
	jar, r, s := newServer(t)
	testaccrualrequests(t, s.Config)
	ts := httptest.NewServer(r)
	defer ts.Close()
	user := models.User{
		Name:     "user111",
		Password: "password111",
	}
	body := userReq(t, user)
	response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
		"Content-Type": "application/json",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	response, _ = testRequest(t, ts, jar, http.MethodPost, "/api/user/orders", "123455", map[string]string{"Content-type": "text/plain"})
	defer response.Body.Close()
	assert.Equal(t, http.StatusAccepted, response.StatusCode)
	time.Sleep(15 * time.Second)
	type args struct {
		Order string  `json:"order"`
		Sum   float32 `json:"sum"`
	}
	tests := []struct {
		name string
		user string
		arg  args
		want int
	}{
		{
			name: "Valid sum and order",
			user: "user111",
			arg: args{
				Order: "84410807816",
				Sum:   1,
			},
			want: http.StatusOK,
		},
		{
			name: "Invalid order",
			user: "user111",
			arg: args{
				Order: "84410803816",
				Sum:   0,
			},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "Invalid sum",
			user: "user111",
			arg: args{
				Order: "577277243060172",
				Sum:   100000,
			},
			want: http.StatusPaymentRequired,
		},
		{
			name: "Invalid user",
			user: "user11111",
			arg: args{
				Order: "",
				Sum:   5,
			},
			want: http.StatusUnauthorized,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Invalid user" {
				jar, _ = cookiejar.New(nil)
			}
			body, err := json.Marshal(tt.arg)
			require.NoError(t, err)
			log.Debug().Msgf("Generated json: %v", string(body))
			balance, err := s.db.GetBalance(tt.user)
			log.Debug().Msgf("Current balance: %v", balance)
			response, _ = testRequest(t, ts, jar, http.MethodPost, "/api/user/balance/withdraw", string(body), map[string]string{"Content-type": "application/json"})
			defer response.Body.Close()
			assert.Equal(t, tt.want, response.StatusCode)
		})
	}
	err = s.db.DeleteContent("orders")
	require.NoError(t, err)
	err = s.db.DeleteContent("balance")
	require.NoError(t, err)
	err = s.db.DeleteContent("withdraws")
	require.NoError(t, err)
	err = s.db.DeleteContent("users")
	require.NoError(t, err)
}

func TestGophermart_getBalanceWithdraw(t *testing.T) {
	type postWithdrawn struct {
		Order string  `json:"order"`
		Sum   float32 `json:"sum"`
	}
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	cmd := exec.Command("C:\\Users\\пользователь\\Documents\\Education\\gophermart\\cmd\\accrual\\accrual_windows_amd64.exe")
	err := cmd.Start()
	require.NoError(t, err)
	defer cmd.Process.Kill()
	time.Sleep(10 * time.Second)
	log.Debug().Msgf("Accrual stated with pid %v", cmd.Process.Pid)
	require.NotZero(t, cmd.Process.Pid)
	jar, r, s := newServer(t)
	testaccrualrequests(t, s.Config)
	ts := httptest.NewServer(r)
	defer ts.Close()
	user := models.User{
		Name:     "user112",
		Password: "password112",
	}
	body := userReq(t, user)
	response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
		"Content-Type": "application/json",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	user = models.User{
		Name:     "user111",
		Password: "password111",
	}
	body = userReq(t, user)
	response, _ = testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
		"Content-Type": "application/json",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	response, _ = testRequest(t, ts, jar, http.MethodPost, "/api/user/orders", "123455", map[string]string{"Content-type": "text/plain"})
	defer response.Body.Close()
	assert.Equal(t, http.StatusAccepted, response.StatusCode)
	time.Sleep(15 * time.Second)
	body = userReq(t, postWithdrawn{Order: "84410807816", Sum: 1})
	log.Debug().Msgf("Body: %v", body)
	response, _ = testRequest(t, ts, jar, http.MethodPost, "/api/user/balance/withdraw", string(body), map[string]string{"Content-type": "application/json"})
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	body = userReq(t, postWithdrawn{Order: "577277243060172", Sum: 1000})
	log.Debug().Msgf("Body: %v", body)
	response, _ = testRequest(t, ts, jar, http.MethodPost, "/api/user/balance/withdraw", string(body), map[string]string{"Content-type": "application/json"})
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	type wanted struct {
		code      int
		withdraws []models.Withdraw
		cType     string
		content   []models.Withdraw
	}
	tests := []struct {
		name string
		want wanted
	}{
		{
			name: "Not empty withdraws",
			want: wanted{
				code:  http.StatusOK,
				cType: "application/json",
				withdraws: []models.Withdraw{
					{
						Number:   "577277243060172",
						Withdraw: 1000,
					},
					{
						Number:   "84410807816",
						Withdraw: 1,
					},
				},
			},
		},
		{
			name: "Empty withdraws",
			want: wanted{
				code: http.StatusNoContent,
			},
		},
		{
			name: "Invalid user",
			want: wanted{
				code: http.StatusUnauthorized,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Invalid user" {
				jar, _ = cookiejar.New(nil)
			}
			if tt.name == "Empty withdraws" {
				jar, _ = cookiejar.New(nil)
				user := models.User{
					Name:     "user112",
					Password: "password112",
				}
				body = userReq(t, user)
				response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/login", body, map[string]string{"Content-Type": "application/json"})
				defer response.Body.Close()
			}
			response, got := testRequest(t, ts, jar, http.MethodGet, "/api/user/balance/withdraw", "", map[string]string{})
			defer response.Body.Close()
			assert.Equal(t, tt.want.code, response.StatusCode)
			if tt.want.code == http.StatusOK {
				g := make([]models.Withdraw, 0)
				json.Unmarshal([]byte(got), &g)
				for i, w := range g {
					assert.Equal(t, tt.want.withdraws[i].Number, w.Number)
					assert.Equal(t, tt.want.withdraws[i].Withdraw, w.Withdraw)
				}
			}
		})
	}
	err = s.db.DeleteContent("orders")
	require.NoError(t, err)
	err = s.db.DeleteContent("balance")
	require.NoError(t, err)
	err = s.db.DeleteContent("withdraws")
	require.NoError(t, err)
	err = s.db.DeleteContent("users")
	require.NoError(t, err)
}

func TestGophermart_GzipSupportReq(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	jar, r, s := newServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()
	tests := []struct {
		name  string
		ctype map[string]string
		args  models.User
		want  int
	}{
		{
			name: "Register new user",
			ctype: map[string]string{
				"Content-Type":     "application/json",
				"Content-Encoding": "gzip",
			},
			args: models.User{
				Name:     "user111",
				Password: "password111",
			},
			want: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := userReq(t, tt.args)
			response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, tt.ctype)
			defer response.Body.Close()
			assert.Equal(t, tt.want, response.StatusCode)
		})
	}
	err := s.db.DeleteContent("orders")
	require.NoError(t, err)
	err = s.db.DeleteContent("balance")
	require.NoError(t, err)
	err = s.db.DeleteContent("withdraws")
	require.NoError(t, err)
	err = s.db.DeleteContent("users")
	require.NoError(t, err)
}

func TestGophermart_GzipSupportResp(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	jar, r, s := newServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()
	user := models.User{
		Name:     "user111",
		Password: "password111",
	}
	body := userReq(t, user)
	response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
		"Content-Type": "application/json",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	response, _ = testRequest(t, ts, jar, http.MethodPost, "/api/user/orders", "123455", map[string]string{
		"Content-Type": "text/plain",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusAccepted, response.StatusCode)
	tests := []struct {
		name string
		user string
		want []models.Order
		code int
	}{
		{
			name: "Get valid orders",
			user: "user1",
			want: []models.Order{
				{
					Number:  "123455",
					Status:  "PROCESSED",
					AccRual: 0,
				},
			},
			code: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Get unauthorized orders" {
				jar, _ = cookiejar.New(nil)
			}
			if tt.name == "Get empty orders" {
				user := models.User{
					Name:     "user112",
					Password: "password112",
				}
				body := userReq(t, user)
				response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
					"Content-Type":    "application/json",
					"Accept-Encoding": "gzip",
				})
				defer response.Body.Close()
			}
			response, _ := testRequest(t, ts, jar, http.MethodGet, "/api/user/orders", "", map[string]string{})
			defer response.Body.Close()
			assert.Equal(t, tt.code, response.StatusCode)
		})
	}
	err := s.db.DeleteContent("orders")
	require.NoError(t, err)
	err = s.db.DeleteContent("balance")
	require.NoError(t, err)
	err = s.db.DeleteContent("withdraws")
	require.NoError(t, err)
	err = s.db.DeleteContent("users")
	require.NoError(t, err)
}
