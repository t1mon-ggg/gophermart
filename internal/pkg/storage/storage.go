package storage

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/t1mon-ggg/gophermart/internal/pkg/models"
)

//Prosgres

const (
	dbSchema = `
	CREATE TABLE IF NOT EXISTS users (
		id int4 NOT NULL GENERATED ALWAYS AS IDENTITY,
		"name" varchar NOT NULL UNIQUE,
		"password" varchar NOT NULL,
		"random_iv" varchar NOT NULL,
		CONSTRAINT users_id_pk PRIMARY KEY (id)
	);
	`
	createUser = `INSERT INTO public.users ("name","password","random_iv") VALUES ($1,$2,$3);`
	getUser    = `SELECT "password", "random_iv" from "users" where "name" = $1`
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
	db, err := pgxpool.Connect(context.Background(), s)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	log.Debug().Msg("Connection to database successfuly created")
	return db, nil
}

func (s *Database) create() error {
	_, err := s.conn.Query(context.Background(), dbSchema)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	log.Debug().Msg("Tables already exists or successfully created")
	return nil
}

func (s *Database) CreateUser(login, password, v string) error {
	_, err := s.conn.Exec(context.Background(), createUser, login, password, v)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	log.Debug().Msgf("User %s created", login)
	return nil
}

func (s *Database) GetUser(login string) (models.User, error) {
	user := models.User{}
	var password string
	var random string
	err := s.conn.QueryRow(context.Background(), getUser, login).Scan(&password, &random)
	if err != nil {
		log.Error().Err(err).Msg("")
		return user, err
	}
	log.Debug().Msgf("Sql result: password is %s, random is %s", password, random)
	user.Name = login
	user.Password = password
	user.Random = random
	log.Debug().Msgf("Found user %s with password %s and random %s", user.Name, user.Password, user.Random)
	return user, nil
}
