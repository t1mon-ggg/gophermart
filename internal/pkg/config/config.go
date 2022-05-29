package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/caarlos0/env"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

//Config - struct for handling configuration
type Config struct {
	Bind      string `env:"RUN_ADDRESS"`
	DBPath    string `env:"DATABASE_URI"`
	AccSystem string `env:"ACCRUAL_SYSTEM_ADDRESS"`
}

var sublog = log.With().Str("component", "config").Logger()

//NewConfig - выделение памяти для новой конфигурации
func New() *Config {
	s := Config{
		DBPath:    "postgresql://postgres:admin@127.0.0.1:5432/gophermart?sslmode=disable", //This is default value of DATABASE_URI
		Bind:      "127.0.0.1:8081",                                                        //This is default value of RUN_ADDRESS
		AccSystem: "http://127.0.0.1:8080",                                                 //This is default value of ACCRUAL_SYSTEM_ADDRESS
	}
	err := s.readEnv()
	if err != nil {
		sublog.Error().Err(err).Msg("Critical error whire reading ENV. Quitting")
		os.Exit(1)
	}
	s.readCli()
	sublog.Log().Msgf("The configured log level is set to %s", zerolog.GlobalLevel().String())
	return &s
}

//isFlagPassed - checking the using of the flag
func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

//ReadEnv - чтение переменных окружения
func (cfg *Config) readEnv() error {
	var c Config
	err := env.Parse(&c)
	if err != nil {
		return err
	}
	if c.DBPath != "" {
		cfg.DBPath = c.DBPath
	}
	if c.Bind != "" {
		cfg.Bind = c.Bind
	}
	if c.AccSystem != "" {
		cfg.AccSystem = c.AccSystem
	}
	return nil
}

//flags - map for flag iterations
var flags = map[string]string{
	"a":     "RUN_ADDRESS",
	"d":     "DATABASE_URI",
	"r":     "ACCRUAL_SYSTEM_ADDRESS",
	"debug": "DEBUG",
	"l":     "LOG_LEVEL",
}

//Configuring flags
var path = flag.String("a", "", fmt.Sprintf("reads %s from flags", flags["a"]))
var bind = flag.String("d", "", fmt.Sprintf("reads %s from flags", flags["d"]))
var accPath = flag.String("r", "", fmt.Sprintf("reads %s from flags", flags["r"]))
var debug = flag.Bool("debug", false, "set log level to debug. overwrite other levels")
var level = flag.Int("l", int(zerolog.ErrorLevel), "set log level")

//ReadCli - чтение флагов командной строки
func (cfg *Config) readCli() {
	flag.Parse()
	for flag, info := range flags {
		if isFlagPassed(flag) {
			switch info {
			case "RUN_ADDRESS":
				cfg.Bind = *bind
			case "DATABASE_URI":
				cfg.DBPath = *path
			case "ACCRUAL_SYSTEM_ADDRESS":
				cfg.AccSystem = *accPath
			case "DEBUG":
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			case "LOG_LEVEL":
				if !isFlagPassed("debug") {
					zerolog.SetGlobalLevel(zerolog.Level(*level))
				}
			}

		}
		if !isFlagPassed("debug") && !isFlagPassed("l") {
			zerolog.SetGlobalLevel(zerolog.Level(*level))
		}
	}
}
