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
	defer resp.Body.Close()

	return resp, string(respBody)
}

func userReq(t *testing.T, u, p string) string {
	user := models.User{}
	user.Name = u
	user.Password = p
	d, err := json.Marshal(user)
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
	jar, r, mart := newServer(t)
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
			body := userReq(t, tt.args.name, tt.args.password)
			response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, tt.ctype)
			defer response.Body.Close()
			assert.Equal(t, tt.want, response.StatusCode)
		})
	}
	err := mart.db.DeleteContent("users")
	require.NoError(t, err)
}

func TestGophermart_postLogin(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	jar, r, mart := newServer(t)
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
	body := userReq(t, user.Name, user.Password)
	response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
		"Content-Type": "application/json",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := userReq(t, tt.args.name, tt.args.password)
			response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/login", body, tt.ctype)
			defer response.Body.Close()
			assert.Equal(t, tt.want, response.StatusCode)
		})
	}
	err := mart.db.DeleteContent("users")
	require.NoError(t, err)
}

func TestGophermart_postOrder(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	jar, r, mart := newServer(t)
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
	body := userReq(t, user.Name, user.Password)
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
				body := userReq(t, user.Name, user.Password)
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
	err := mart.db.DeleteContent("orders")
	require.NoError(t, err)
	err = mart.db.DeleteContent("users")
	require.NoError(t, err)
}

func TestGophermart_getOrders(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	jar, r, mart := newServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()
	user := models.User{
		Name:     "user111",
		Password: "password111",
	}
	body := userReq(t, user.Name, user.Password)
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
					Number:  123455,
					Status:  "NEW",
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
				body := userReq(t, user.Name, user.Password)
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
	err := mart.db.DeleteContent("orders")
	require.NoError(t, err)
	err = mart.db.DeleteContent("users")
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
	time.Sleep(10 * time.Second)
	log.Debug().Msgf("Accrual stated with pid %v", cmd.Process.Pid)
	jar, r, s := newServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()
	user := models.User{
		Name:     "user111",
		Password: "password111",
	}
	body := userReq(t, user.Name, user.Password)
	response, _ := testRequest(t, ts, jar, http.MethodPost, "/api/user/register", body, map[string]string{
		"Content-Type": "application/json",
	})
	defer response.Body.Close()
	assert.Equal(t, http.StatusOK, response.StatusCode)
	response, _ = testRequest(t, ts, jar, http.MethodPost, "/api/user/orders", "123455", map[string]string{"Content-type": "text/plain"})
	defer response.Body.Close()
	assert.Equal(t, http.StatusAccepted, response.StatusCode)
	testaccrualrequests(t, s.Config)
	defer cmd.Process.Kill()
	tests := []struct {
		name  string
		login string
		order int
	}{
		{
			name:  "Valid order",
			login: "user111",
			order: 123455,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.AccrualAPI(tt.login, tt.order)
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
	err = s.db.DeleteContent("users")
	require.NoError(t, err)
}
