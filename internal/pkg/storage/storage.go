package storage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/t1mon-ggg/gophermart/internal/pkg/models"
)

//Postgres

const (
	dbSchema = `
	CREATE TABLE IF NOT EXISTS users (
		id int4 NOT NULL GENERATED ALWAYS AS IDENTITY,
		"name" text NOT NULL UNIQUE,
		"password" text NOT NULL,
		"random_iv" text NOT NULL,
		"balance" float8 NOT NULL DEFAULT 0,
		"withdrawn" float8 NOT NULL DEFAULT 0,
		CONSTRAINT users_id_pk PRIMARY KEY (id)
	);

	CREATE TABLE IF NOT EXISTS public.orders (
		id int4 NOT NULL GENERATED ALWAYS AS IDENTITY,
		"order" text NOT NULL,
		"name" text NOT NULL,
		"status" text NOT NULL DEFAULT 'NEW',
		"uploaded_at" timestamptz NOT NULL,
		"processed_at"  timestamptz,
		"accrual" float8 NOT NULL DEFAULT 0,
		"withdrawn" float8 NOT NULL DEFAULT 0,
		CONSTRAINT orders_fk FOREIGN KEY (name) REFERENCES public.users("name"),
		CONSTRAINT orders_id_pk PRIMARY KEY (id)
	);
	CREATE UNIQUE INDEX IF NOT EXISTS orders_order_user_idx ON public.orders ("order","name");
	CREATE UNIQUE INDEX IF NOT EXISTS orders_order_idx ON public.orders ("order");
	`
	createUser      = `INSERT INTO public.users ("name","password","random_iv") VALUES ($1,$2,$3)`
	getUser         = `SELECT "password", "random_iv" from "users" where "name" = $1`
	createOrder     = `INSERT INTO public.orders ("order","name","uploaded_at") VALUES ($1,$2,$3)`
	getOrders       = `SELECT "order", "status", "accrual", "uploaded_at" from "orders" where "name" = $1 ORDER BY "uploaded_at" DESC`
	getBalance      = `SELECT "balance", "withdrawn" from "users" where "name" = $1`
	updateOrder     = `UPDATE public.orders SET status=$1, accrual=$2, processed_at=$3 WHERE "order" = $4`
	updateBalance   = `UPDATE public.users SET balance=$1, withdrawn=$2 WHERE "name" = $3`
	updateWithdrawn = `UPDATE public.orders SET withdrawn = $1, processed_at = $2  WHERE "order" = $3 AND "name" = $4`
	getWithdrawns   = `SELECT "order", "withdrawn", "processed_at" from "orders" where "name" = $1 ORDER BY "processed_at" DESC`
)

type Database struct {
	conn *pgxpool.Pool
}

var sublog = log.With().Str("component", "storage").Logger()

func New(path string) (*Database, error) {
	db := Database{}
	d, err := open(path)
	if err != nil {
		sublog.Error().Msg("Error while connecting to Posgres database. Quiting")
		return nil, err
	}
	db.conn = d

	err = db.create()
	if err != nil {
		sublog.Error().Msg("Fatal error on table create. Quiting")
		return nil, err
	}
	sublog.Info().Msg("Database object successfully created")
	return &db, nil

}

func open(s string) (*pgxpool.Pool, error) {
	sublog.Info().Msg("Connectiong to database")
	db, err := pgxpool.Connect(context.Background(), s)
	if err != nil {
		sublog.Error().Err(err).Msg("")
		return nil, err
	}
	sublog.Info().Msg("Connection to database successfuly created")
	return db, nil
}

func (s *Database) create() error {
	sublog.Info().Msg("Creating databse scheme")
	_, err := s.conn.Exec(context.Background(), dbSchema)
	if err != nil {
		sublog.Error().Err(err).Msg("")
		return err
	}
	sublog.Info().Msg("Tables already exists or successfully created")
	return nil
}

func (s *Database) CreateUser(login, password, v string) error {
	sublog.Info().Msgf("Creating user %v", login)
	_, err := s.conn.Exec(context.Background(), createUser, login, password, v)
	if err != nil {
		sublog.Error().Err(err).Msg("")
		return err
	}
	sublog.Info().Msgf("User with name '%s' created", login)
	return nil
}

func (s *Database) GetUser(login string) (models.User, error) {
	sublog.Info().Msgf("Requesting user's %v data", login)
	user := models.User{}
	var password string
	var random string
	err := s.conn.QueryRow(context.Background(), getUser, login).Scan(&password, &random)
	if err != nil {
		sublog.Error().Err(err).Msg("")
		return user, err
	}
	sublog.Debug().Msgf("Sql result: password is %s, random is %s", password, random)
	user.Name = login
	user.Password = password
	user.Random = random
	sublog.Debug().Msgf("Found user %s with password %s and random %s", user.Name, user.Password, user.Random)
	return user, nil
}

func (s *Database) GetOrders(login string) ([]models.Order, error) {
	sublog.Info().Msgf("Requesting orders for user %v", login)
	orders := make([]models.Order, 0)
	rows, err := s.conn.Query(context.Background(), getOrders, login)
	if err != nil {
		sublog.Error().Err(err).Msg("")
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		order := models.Order{}
		var number string
		var status string
		var accrual float32
		var upload time.Time
		err = rows.Scan(&number, &status, &accrual, &upload)
		if err != nil {
			sublog.Error().Err(err).Msg("Error while reading rows")
			return nil, err
		}
		order.Number = number
		order.Status = status
		order.AccRual = accrual
		order.Upload = upload
		sublog.Debug().Msgf("Odrer: %v", order)
		orders = append(orders, order)
	}
	sublog.Debug().Msgf("Result order slice: %v", orders)
	return orders, nil
}

func (s *Database) CreateOrder(order, user string) error {
	sublog.Info().Msgf("Creating new order %v", order)
	_, err := s.conn.Exec(context.Background(), createOrder, order, user, time.Now())
	if err != nil {
		sublog.Error().Err(err).Msg("")
		return err
	}
	sublog.Info().Msgf("Order %v created", order)
	return nil
}

func (s *Database) UpdateOrder(order, status string, accrual float32) error {
	sublog.Debug().Msgf("Updating order %v with new status %v and accrual value %v", order, status, accrual)
	_, err := s.conn.Exec(context.Background(), updateOrder, status, accrual, time.Now(), order)
	if err != nil {
		sublog.Debug().Err(err).Msg("")
		return err
	}
	sublog.Info().Msgf("Order %v updated", order)
	return nil
}

func (s *Database) GetBalance(login string) (float32, float32, error) {
	var balance float32
	var withdrawn float32
	sublog.Info().Msgf("Requesting balance for user %v", login)
	err := s.conn.QueryRow(context.Background(), getBalance, login).Scan(&balance, &withdrawn)
	if err != nil {
		sublog.Error().Err(err).Msg("")
		return 0, 0, err
	}
	return balance, withdrawn, nil
}

func (s *Database) UpdateBalance(login string, accrual float32) error {
	sublog.Info().Msg("Updating balance")
	sublog.Debug().Msgf("User is %v and delta is %v", login, accrual)

	sublog.Debug().Msgf("SQL Query: %v", fmt.Sprintf(""))
	balance, withdraw, err := s.GetBalance(login)
	if err != nil {
		sublog.Error().Err(err).Msg("Error in get user balance request")
		return err
	}
	log.Debug().Msgf("Old balance is %v", balance)
	balance += accrual
	if balance < 0 {
		sublog.Error().Msg("Balance is not enough")
		return errors.New("we need to build more ziggurats")
	}
	if accrual < 0 {
		withdraw += float32(math.Abs(float64(accrual)))
	}
	sublog.Debug().Msgf("N/ew balance is %v. New withdrawns is %v", balance, withdraw)
	_, err = s.conn.Exec(context.Background(), updateBalance, balance, withdraw, login)
	if err != nil {
		sublog.Error().Err(err).Msg("Error in update user balance request")
		return err
	}
	sublog.Info().Msgf("User %v updated", login)
	return nil
}

func (s *Database) UpdateWithdrawn(sum float32, login, order string) error {
	sublog.Info().Msg("Updating withdrawn")
	sublog.Debug().Msgf("User is %v. withdrawn sum is %.2f for order %v", login, sum, order)
	sum = sum * -1
	err := s.UpdateBalance(login, sum)
	if err != nil {
		sublog.Info().Msg("Update balance failed")
		return err
	}
	_, err = s.conn.Exec(context.Background(), updateWithdrawn, sum, time.Now(), order, login)
	if err != nil {
		sublog.Error().Err(err).Msg("")
		return err
	}
	sublog.Info().Msg("Update orders withdrawn complete")
	return nil
}

func (s *Database) GetWithdrawns(login string) ([]models.Order, error) {
	sublog.Info().Msgf("Requesting user's %v withdrawns", login)
	orders := make([]models.Order, 0)
	rows, err := s.conn.Query(context.Background(), getWithdrawns, login)
	if err != nil {
		sublog.Error().Err(err).Msg("")
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		order := models.Order{}
		var number string
		var processed time.Time
		var withdrawn float32
		err = rows.Scan(&number, &withdrawn, &processed)
		if err != nil {
			sublog.Error().Err(err).Msg("Error while reading rows")
			return nil, err
		}
		sublog.Debug().Msgf("Withdrawn: %v", order)
		if withdrawn > 0 {
			orders = append(orders, order)
		}
	}
	sublog.Debug().Msgf("Result withdrawn slice: %v", orders)
	return orders, nil
}

func (s *Database) DeleteContent(table string) error {
	_, err := s.conn.Exec(context.Background(), fmt.Sprintf("DELETE from \"%s\"", table))
	if err != nil {
		sublog.Error().Err(err).Msgf("Error while cleaning table %s", table)
		return err
	}
	sublog.Debug().Msgf("Table %s is clean", table)
	return nil
}

func (s *Database) Close() {
	s.conn.Close()
	sublog.Info().Msg("Connection to database closed")
}
