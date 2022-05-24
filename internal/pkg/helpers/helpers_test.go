package helpers

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestSecurePassword(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	tests := []struct {
		name string
		pass string
	}{
		{
			name: "Empty password",
			pass: "",
		},
		{
			name: "Short password",
			pass: "1234",
		},
		{
			name: "Long password",
			pass: "ThisIsLongPassword",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SecurePassword(tt.pass)
			require.NoError(t, err)
		})
	}
}

func TestComparePassword(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	tests := []struct {
		name string
		pass string
		hash string
		want bool
	}{
		{
			name: "Empty password",
			pass: "",
			hash: "$2a$10$zv5AM5yg3YzpvfjGxmCahOh7XMU60dyfchvidcGMRxy/qD81A79bC",
			want: true,
		},
		{
			name: "Short password",
			pass: "1234",
			hash: "$2a$10$EmXb3pf.Y3w1wCM7UXVzrO9UyMLgjdB2fLOje.VgXvzX5un1AGp4O",
			want: true,
		},
		{
			name: "Long password",
			pass: "ThisIsLongPassword",
			hash: "$2a$10$elhDMDuu.j.BfKxoML36w.mrAu.8nIr/M1KIv8pqLtcavSUUsOfha",
			want: true,
		},
		{
			name: "Wrong long password",
			pass: "ThisIsLongPassword",
			hash: "$2a$10$EmXb3pf.Y3w1wCM7UXVzrO9UyMLgjdB2fLOje.VgXvzX5un1AGp4O",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComparePassword(tt.pass, tt.hash)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateCookieValue(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	type args struct {
		user   string
		hash   string
		ip     string
		random string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test1",
			args: args{
				user:   "user1",
				hash:   "$2a$10$elhDMDuu.j.BfKxoML36w.mrAu.8nIr/M1KIv8pqLtcavSUUsOfha",
				ip:     "127.0.0.1",
				random: "1234567890",
			},
			want: "80f6797f02d8ee32a9c96e3ec6cfe808:41d6d9af91c5b75b68b446a63a94eebc348de16ec06f67be36dea6e64ce51b60",
		},
		{
			name: "test2",
			args: args{
				user:   "user2",
				hash:   "$2a$10$EmXb3pf.Y3w1wCM7UXVzrO9UyMLgjdB2fLOje.VgXvzX5un1AGp4O",
				ip:     "192.168.10.20",
				random: "1234567890",
			},
			want: "a21214785bda2631ca78ca4a547255ee:db8d617b5e07b6fe42efb9436fa00d149f308d9276f03f5f4377d171d65a858c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateCookieValue(tt.args.user, tt.args.hash, tt.args.ip, tt.args.random)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCompareCookie(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	type args struct {
		cookie string
		user   string
		hash   string
		ip     string
		random string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Valid cookie",
			args: args{
				cookie: "a21214785bda2631ca78ca4a547255ee:db8d617b5e07b6fe42efb9436fa00d149f308d9276f03f5f4377d171d65a858c",
				user:   "user2",
				hash:   "$2a$10$EmXb3pf.Y3w1wCM7UXVzrO9UyMLgjdB2fLOje.VgXvzX5un1AGp4O",
				ip:     "192.168.10.20",
				random: "1234567890",
			},
			want: true,
		},
		{
			name: "Data missmatch",
			args: args{
				cookie: "a21214785bda2631ca78ca4a547255ee:db8d617b5e07b6fe42efb9436fa00d149f308d9276f03f5f4377d171d65a858c",
				user:   "user1",
				hash:   "$2a$10$elhDMDuu.j.BfKxoML36w.mrAu.8nIr/M1KIv8pqLtcavSUUsOfha",
				ip:     "127.0.0.1",
				random: "1234567890",
			},
			want: false,
		},
		{
			name: "Sign missmatch",
			args: args{
				cookie: "a21214785bda2631ca78ca4a547255ee:db8d6123e07b6fe42efb9436fa00d149f308d9276f03f5f4377d171d65a858c",
				user:   "user2",
				hash:   "$2a$10$EmXb3pf.Y3w1wCM7UXVzrO9UyMLgjdB2fLOje.VgXvzX5un1AGp4O",
				ip:     "192.168.10.20",
				random: "1234567890",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareCookie(tt.args.cookie, tt.args.user, tt.args.hash, tt.args.ip, tt.args.random)
			require.Equal(t, tt.want, got)
		})
	}
}
