package helpers

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"net/http"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/neonxp/checksum"
	"github.com/neonxp/checksum/luhn"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

const (
	letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" //Alphabet for random generation
)

var sublog = log.With().Str("component", "helper").Logger()

//Generate bcrypt password value
func SecurePassword(s string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(s), 10)
	if err != nil {
		sublog.Error().Err(err).Msg("")
		return "", err
	}
	sublog.Debug().Msgf("Password hash created: %s", string(hash))
	return string(hash), nil
}

//Compare recieved password with stored value
func ComparePassword(s, h string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(h), []byte(s))
	if err == nil {
		sublog.Debug().Msg("Password correct")
		return true
	}
	sublog.Debug().Msg("Password incorrect")
	return false
}

/*
GenerateCookieValue - генерация значения cookie

u - username,

h - hashed password,

ip - user ip address,

r - random string
*/
func GenerateCookieValue(u, h, ip, r string) string {
	data := []byte(u + h + ip)
	d := md5.Sum(data)
	digest := make([]byte, 0)
	for i := range d {
		digest = append(digest, d[i])
	}
	sublog.Debug().Msgf("Cookie data is %s", hex.EncodeToString(digest))
	hash := hmac.New(sha256.New, []byte(r))
	hash.Write(digest)
	sign := hash.Sum(nil)
	sublog.Debug().Msgf("Cookie sign is %s", hex.EncodeToString(sign))
	value := hex.EncodeToString(digest) + ":" + hex.EncodeToString(sign)
	sublog.Debug().Msgf("Cookie value is %s", value)
	return value
}

/*
CompareCookie - проверка значения cookie

c - cookie value,

u - username,

h - hashed password,

ip - user ip address,

r - random string
*/
func CompareCookie(c, u, h, ip, r string) bool {
	cc := strings.Split(c, ":")
	sublog.Debug().Msgf("cookie data: %s, cookie sign: %s", cc[0], cc[1])
	data := []byte(u + h + ip)
	d := md5.Sum(data)
	digest := make([]byte, 0)
	for i := range d {
		digest = append(digest, d[i])
	}
	sublog.Debug().Msgf("Cookie data is %s", hex.EncodeToString(digest))
	if hex.EncodeToString(digest) != cc[0] {
		sublog.Debug().Msg("Cookie is invalid. Data missmatch")
		return false
	}

	hash := hmac.New(sha256.New, []byte(r))
	hash.Write(digest)
	sign := hash.Sum(nil)
	sublog.Debug().Msgf("Cookie sign is %s", hex.EncodeToString(sign))
	if hex.EncodeToString(sign) != cc[1] {
		sublog.Debug().Msg("Cookie is invalid. Sign missmatch")
		return false
	}
	sublog.Debug().Msg("Cookie is valid")
	return true
}

//RandStringRunes - generate random string with custom lenght
func RandStringRunes(n int) string {
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			sublog.Error().Err(err).Msg("Error in random generator. Ignoring and return empty string")
			return ""
		}
		b[i] = letters[num.Int64()]
	}
	sublog.Debug().Msgf("Random generation complete. Generated string is '%s'", string(b))
	return string(b)
}

//UserConflict - checking the sql error for unique violation of username
func UserConflict(err error) bool {
	sublog.Debug().Msg("Check unique user error")
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		sublog.Debug().Msgf("Error message: %s", pgErr.Message)
		sublog.Debug().Msgf("Error code: %s", pgErr.Code)
		if pgErr.Code == "23505" {
			return true
		}
	}
	return false
}

//OrderUnique - checking the sql error for unique violation of order number
func OrderUnique(err error) bool {
	sublog.Debug().Msg("Check unique user error")
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		sublog.Debug().Msgf("Error message: %s", pgErr.Message)
		sublog.Debug().Msgf("Error code: %s", pgErr.Code)
		if pgErr.Code == "23505" {
			if strings.Contains(pgErr.Message, "orders_order_idx") {
				return true
			}
		}
	}
	return false
}

//OrderExists - checking the sql error for unique violation of order number and username pair
func OrderExists(err error) bool {
	sublog.Debug().Msg("Check unique user's order error")
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		sublog.Debug().Msgf("Error message: %s", pgErr.Message)
		sublog.Debug().Msgf("Error code: %s", pgErr.Code)
		if pgErr.Code == "23505" {
			if strings.Contains(pgErr.Message, "orders_order_user_idx") {
				return true
			}
		}
	}
	return false
}

//EmptyRow - cheking sql error for emtry rows result set
func EmptyRow(err error) bool {
	sublog.Debug().Msg("Check empty row error")
	return err.Error() == "no rows in result set"
}

//SetCookie - writing new cookie to web response
func SetCookie(w http.ResponseWriter, name, value string) {
	sublog.Debug().Msgf("Creating new cookie %v", name)
	cookie := http.Cookie{
		Name:   name,
		Value:  value,
		MaxAge: 0,
		Path:   "/",
	}
	http.SetCookie(w, &cookie)
}

//GetUser - getting username from request cookies
func GetUser(r *http.Request) (string, error) {
	sublog.Debug().Msg("Reading user name from cookie")
	cookie, err := r.Cookie("username")
	if err != nil {
		sublog.Error().Err(err)
		return "", err
	}
	username := cookie.Value
	sublog.Debug().Msgf("Username from cookie is %v", username)
	return username, nil
}

//CheckOrder - checking order number by luhn algorithm
func CheckOrder(s []byte) bool {
	sublog.Debug().Msg("Checking order number wirh luhn algorithm")
	str := string(s)
	err := luhn.Check(str)
	if err != nil {
		switch err {
		case checksum.ErrInvalidNumber:
			sublog.Info().Msg("Invalid order number")
			return false
		case checksum.ErrInvalidChecksum:
			sublog.Info().Msg("Invalid order checksum")
			return false
		}
	}
	sublog.Info().Msg("Order number is valid")
	return true
}

//BalanceTooLow - checking the error for a low balance user error
func BalanceTooLow(err error) bool {
	sublog.Debug().Msg("Check low balance error")
	return err.Error() == "we need to build more ziggurats"
}
