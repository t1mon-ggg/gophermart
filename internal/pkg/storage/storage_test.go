package storage

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t1mon-ggg/gophermart/internal/pkg/models"
)

func Test_open(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "Valid path",
			path: "postgresql://postgres:admin@127.0.0.1:5432/gophermart?sslmode=disable",
			want: true,
		},
		{
			name: "Invalid path",
			path: "postgresql://postgres:admin@127.0.0.1:5432/invalid?sslmode=disable",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := false
			_, err := open(tt.path)
			if err == nil {
				e = true
				require.Equal(t, tt.want, e)
			} else {
				require.Equal(t, tt.want, e)
			}
		})
	}
}

func TestDatabase_create(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	tests := []struct {
		name string
		path string
	}{
		{
			name: "SQL Schema test",
			path: "postgresql://postgres:admin@127.0.0.1:5432/gophermart?sslmode=disable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Database{}
			db, err := open(tt.path)
			require.NoError(t, err)
			d.conn = db
			err = (&d).create()
			require.NoError(t, err)
		})
	}
}

func TestNew(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "Correct path",
			path: "postgresql://postgres:admin@127.0.0.1:5432/gophermart?sslmode=disable",
			want: true,
		},
		{
			name: "Incorrect path",
			path: "postgresql://postgres:admin@127.0.0.1:5432/invalid?sslmode=disable",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := true
			_, err := New(tt.path)
			if err != nil {
				e = false
			}
			require.Equal(t, tt.want, e)
		})
	}
}

func TestDatabase_createUser(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	type args struct {
		login    string
		password string
		vector   string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Create user 1",
			args: args{
				login:    "username1",
				password: "password1",
				vector:   "iv1",
			},
			want: true,
		},
		{
			name: "Create user 2",
			args: args{
				login:    "username2",
				password: "password2",
				vector:   "iv2",
			},
			want: true,
		},
		{
			name: "Create user duplicate",
			args: args{
				login:    "username1",
				password: "password3",
				vector:   "iv3",
			},
			want: false,
		},
	}
	db, err := New("postgresql://postgres:admin@127.0.0.1:5432/gophermart?sslmode=disable")
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := true
			err := db.CreateUser(tt.args.login, tt.args.password, tt.args.vector)
			if err != nil {
				e = false
			}
			require.Equal(t, tt.want, e)
		})
	}
	err = db.DeleteContent("orders")
	require.NoError(t, err)
	err = db.DeleteContent("users")
	require.NoError(t, err)
}

func TestDatabase_getUser(t *testing.T) {
	tests := []struct {
		name string
		args models.User
		want bool
	}{
		{
			name: "Existing user",
			args: models.User{
				Name:     "user1",
				Password: "password1",
				Random:   "random1",
			},
			want: true,
		},
		{
			name: "Not existing user",
			args: models.User{
				Name: "user2",
			},
			want: false,
		},
	}
	db, err := New("postgresql://postgres:admin@127.0.0.1:5432/gophermart?sslmode=disable")
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.Name == "user1" {
				err = db.CreateUser(tt.args.Name, tt.args.Password, tt.args.Random)
				require.NoError(t, err)
			}
			e := true
			user, err := db.GetUser(tt.args.Name)
			if err != nil {
				e = false
			}
			require.Equal(t, tt.want, e)
			if tt.args.Name == "user1" {
				require.Equal(t, tt.args, user)
			}
		})
	}
	err = db.DeleteContent("orders")
	require.NoError(t, err)
	err = db.DeleteContent("users")
	require.NoError(t, err)
}

func TestDatabase_CreateOrder(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	type args struct {
		order string
		user  string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "valid new order",
			args: args{
				order: "12345678",
				user:  "user1",
			},
			want: true,
		},
		{
			name: "Not unique order",
			args: args{
				order: "12345678",
				user:  "user2",
			},
			want: false,
		},
		{
			name: "Order already created by user",
			args: args{
				order: "12345678",
				user:  "user1",
			},
			want: false,
		},
	}
	db, err := New("postgresql://postgres:admin@127.0.0.1:5432/gophermart?sslmode=disable")
	require.NoError(t, err)
	err = db.CreateUser("user1", "password", "random1")
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := true
			err := db.CreateOrder(tt.args.order, tt.args.user)
			if err != nil {
				e = false
			}
			assert.Equal(t, tt.want, e)
		})
	}
	err = db.DeleteContent("orders")
	require.NoError(t, err)
	err = db.DeleteContent("users")
	require.NoError(t, err)

}
