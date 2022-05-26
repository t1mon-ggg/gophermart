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
	letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

//Generate bcrypt password value
func SecurePassword(s string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(s), 10)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	log.Debug().Msgf("Password hash created: %s", string(hash))
	return string(hash), nil
}

//Compare recieved password with stored value
func ComparePassword(s, h string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(h), []byte(s))
	if err == nil {
		log.Debug().Msg("Password correct")
		return true
	}
	log.Debug().Msg("Password incorrect")
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
	log.Debug().Msgf("Cookie data is %s", hex.EncodeToString(digest))
	hash := hmac.New(sha256.New, []byte(r))
	hash.Write(digest)
	sign := hash.Sum(nil)
	log.Debug().Msgf("Cookie sign is %s", hex.EncodeToString(sign))
	value := hex.EncodeToString(digest) + ":" + hex.EncodeToString(sign)
	log.Debug().Msgf("Cookie value is %s", value)
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
	log.Debug().Msgf("cookie data: %s, cookie sign: %s", cc[0], cc[1])
	data := []byte(u + h + ip)
	d := md5.Sum(data)
	digest := make([]byte, 0)
	for i := range d {
		digest = append(digest, d[i])
	}
	log.Debug().Msgf("Cookie data is %s", hex.EncodeToString(digest))
	if hex.EncodeToString(digest) != cc[0] {
		log.Debug().Msg("Cookie is invalid. Data missmatch")
		return false
	}

	hash := hmac.New(sha256.New, []byte(r))
	hash.Write(digest)
	sign := hash.Sum(nil)
	log.Debug().Msgf("Cookie sign is %s", hex.EncodeToString(sign))
	if hex.EncodeToString(sign) != cc[1] {
		log.Debug().Msg("Cookie is invalid. Sign missmatch")
		return false
	}
	log.Debug().Msg("Cookie is valid")
	return true
}

func RandStringRunes(n int) string {
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			log.Error().Err(err).Msg("Error in random generator. Ignoring and return empty string")
			return ""
		}
		b[i] = letters[num.Int64()]
	}
	log.Debug().Msgf("Random generation complete. Generated string is '%s'", string(b))
	return string(b)
}

func UserConflict(err error) bool {
	log.Debug().Msg("Check unique user error")
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		log.Debug().Msgf("Error message: %s", pgErr.Message)
		log.Debug().Msgf("Error code: %s", pgErr.Code)
		if pgErr.Code == "23505" {
			return true
		}
	}
	return false
}

func OrderUnique(err error) bool {
	log.Debug().Msg("Check unique user error")
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		log.Debug().Msgf("Error message: %s", pgErr.Message)
		log.Debug().Msgf("Error code: %s", pgErr.Code)
		if pgErr.Code == "23505" {
			if strings.Contains(pgErr.Message, "orders_order_idx") {
				return true
			}
		}
	}
	return false
}

func OrderExists(err error) bool {
	log.Debug().Msg("Check unique user's order error")
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		log.Debug().Msgf("Error message: %s", pgErr.Message)
		log.Debug().Msgf("Error code: %s", pgErr.Code)
		if pgErr.Code == "23505" {
			if strings.Contains(pgErr.Message, "orders_order_user_idx") {
				return true
			}
		}
	}
	return false
}

func UserNotFound(err error) bool {
	log.Debug().Msg("Check empty row error")
	return err.Error() == "no rows in result set"
}

func SetCookie(w http.ResponseWriter, name, value string) {
	cookie := http.Cookie{
		Name:   name,
		Value:  value,
		MaxAge: 0,
		Path:   "/",
	}
	http.SetCookie(w, &cookie)
}

func GetUser(r *http.Request) (string, error) {
	cookie, err := r.Cookie("username")
	if err != nil {
		log.Error().Err(err)
		return "", err
	}
	username := cookie.Value
	log.Debug().Msgf("Username from cookie is %v", username)
	return username, nil
}

func CheckOrder(s []byte) bool {
	str := string(s)
	err := luhn.Check(str)
	if err != nil {
		switch err {
		case checksum.ErrInvalidNumber:
			log.Error().Msg("Invalid order number")
			return false
		case checksum.ErrInvalidChecksum:
			log.Error().Msg("Invalid order checksum")
			return false
		}
	}
	log.Info().Msg("Order number is valid")
	return true
}
