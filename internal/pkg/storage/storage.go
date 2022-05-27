package storage

import (
	"context"
	"errors"
	"fmt"
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
		"name" varchar NOT NULL UNIQUE,
		"password" varchar NOT NULL,
		"random_iv" varchar NOT NULL,
		"balance" float8 NOT NULL DEFAULT 0,
		CONSTRAINT users_id_pk PRIMARY KEY (id)
	);

	CREATE TABLE IF NOT EXISTS public.orders (
		id int4 NOT NULL GENERATED ALWAYS AS IDENTITY,
		"order" int8 NOT NULL,
		"name" text NOT NULL,
		"status" text NOT NULL DEFAULT 'NEW',
		"uploaded_at" timestamptz NOT NULL,
		"accrual" float8 NOT NULL DEFAULT 0,
		CONSTRAINT orders_fk FOREIGN KEY (name) REFERENCES public.users("name"),
		CONSTRAINT orders_id_pk PRIMARY KEY (id)
	);
	CREATE UNIQUE INDEX IF NOT EXISTS orders_order_user_idx ON public.orders ("order","name");
	CREATE UNIQUE INDEX IF NOT EXISTS orders_order_idx ON public.orders ("order");
	`
	createUser    = `INSERT INTO public.users ("name","password","random_iv") VALUES ($1,$2,$3)`
	getUser       = `SELECT "password", "random_iv" from "users" where "name" = $1`
	createOrder   = `INSERT INTO public.orders ("order","name","uploaded_at") VALUES ($1,$2,$3)`
	getOrders     = `SELECT "order", "status", "accrual", "uploaded_at" from "orders" where "name" = $1 ORDER BY "uploaded_at" DESC`
	getBalance    = `SELECT "balance" from "users" where "name" = $1`
	updateOrder   = `UPDATE public.orders SET status=$1, accrual=$2	WHERE "order" = $3`
	updateBalance = `UPDATE public.users SET balance=$1 WHERE "name" = $2`
)

type Database struct {
	conn *pgxpool.Pool
}

func New(path string) (*Database, error) {
	db := Database{}
	d, err := open(path)
	if err != nil {
		log.Error().Msg("Error while connecting to Posgres database. Quiting")
		return nil, err
	}
	db.conn = d

	err = db.create()
	if err != nil {
		log.Error().Msg("Fatal error on table create. Quiting")
		return nil, err
	}
	log.Debug().Msg("Object 'Database' successfully created")
	return &db, nil

}

func open(s string) (*pgxpool.Pool, error) {
	log.Debug().Msg("Connectiong to database")
	db, err := pgxpool.Connect(context.Background(), s)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	log.Debug().Msg("Connection to database successfuly created")
	return db, nil
}

func (s *Database) create() error {
	log.Debug().Msg("Creating databse scheme")
	_, err := s.conn.Exec(context.Background(), dbSchema)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	log.Debug().Msg("Tables already exists or successfully created")
	return nil
}

func (s *Database) CreateUser(login, password, v string) error {
	log.Debug().Msgf("Creating user %v", login)
	_, err := s.conn.Exec(context.Background(), createUser, login, password, v)
	if err != nil {
		log.Debug().Err(err).Msg("")
		return err
	}
	log.Debug().Msgf("User %s created", login)
	return nil
}

func (s *Database) GetUser(login string) (models.User, error) {
	log.Debug().Msgf("Requesting user's %v data", login)
	user := models.User{}
	var password string
	var random string
	err := s.conn.QueryRow(context.Background(), getUser, login).Scan(&password, &random)
	if err != nil {
		log.Debug().Err(err).Msg("")
		return user, err
	}
	log.Debug().Msgf("Sql result: password is %s, random is %s", password, random)
	user.Name = login
	user.Password = password
	user.Random = random
	log.Debug().Msgf("Found user %s with password %s and random %s", user.Name, user.Password, user.Random)
	return user, nil
}

func (s *Database) GetOrders(login string) ([]models.Order, error) {
	log.Debug().Msgf("Requesting orders for user %v", login)
	orders := make([]models.Order, 0)
	rows, err := s.conn.Query(context.Background(), getOrders, login)
	if err != nil {
		log.Debug().Err(err).Msg("")
		return orders, err
	}
	defer rows.Close()
	for rows.Next() {
		order := models.Order{}
		var number int
		var status string
		var accrual float32
		var upload time.Time
		err = rows.Scan(&number, &status, &accrual, &upload)
		if err != nil {
			log.Error().Err(err).Msg("Error while reading rows")
			return orders, err
		}
		order.Number = number
		order.Status = status
		order.AccRual = accrual
		order.Upload = upload
		orders = append(orders, order)
	}
	return orders, nil
}

func (s *Database) CreateOrder(order int, user string) error {
	log.Debug().Msgf("Creating new order %v", order)
	_, err := s.conn.Exec(context.Background(), createOrder, order, user, time.Now())
	if err != nil {
		log.Debug().Err(err).Msg("")
		return err
	}
	log.Debug().Msgf("Order %v created", order)
	return nil
}

func (s *Database) UpdateOrder(order int, status string, accrual float32) error {
	log.Debug().Msgf("Updating order %v with new status %v and accrual value %v", order, status, accrual)
	_, err := s.conn.Exec(context.Background(), updateOrder, status, accrual, order)
	if err != nil {
		log.Debug().Err(err).Msg("")
		return err
	}
	log.Debug().Msgf("Order %v updated", order)
	return nil
}

func (s *Database) GetBalance(login string) (float32, error) {
	var balance float32
	log.Debug().Msgf("Requesting balance for user %v", login)
	err := s.conn.QueryRow(context.Background(), getBalance, login).Scan(&balance)
	if err != nil {
		log.Debug().Err(err).Msg("")
		return 0, err
	}
	return balance, nil
}

func (s *Database) UpdateBalance(login string, accrual float32) error {
	balance, err := s.GetBalance(login)
	if err != nil {
		log.Error().Err(err).Msg("Error in get user balance request")
		return err
	}
	balance += accrual
	if balance < 0 {
		log.Error().Msg("Balance is not enough")
		return errors.New("We need to build more ziggurats")
	}
	log.Debug().Msgf("Updating balance for user %v with new balance %v", login, balance)
	_, err = s.conn.Exec(context.Background(), updateBalance, balance, login)
	if err != nil {
		log.Debug().Err(err).Msg("Error in update user balance request")
		return err
	}
	log.Debug().Msgf("User %v updated", login)
	return nil
}

func (s *Database) DeleteContent(table string) error {
	_, err := s.conn.Exec(context.Background(), fmt.Sprintf("DELETE from \"%s\"", table))
	if err != nil {
		log.Error().Err(err).Msgf("Error while cleaning table %s", table)
		return err
	}
	log.Debug().Msgf("Table %s is clean", table)
	return nil
}

func (s *Database) Close() {
	s.conn.Close()
	log.Debug().Msg("Connection to database closed")
}
