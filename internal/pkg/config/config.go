package config

import (
	"flag"
	"log"

	"github.com/caarlos0/env"
	"github.com/rs/zerolog"
)

type Config struct {
	Bind      string `env:"RUN_ADDRESS"`
	DBPath    string `env:"DATABASE_URI"`
	AccSystem string `env:"ACCRUAL_SYSTEM_ADDRESS"`
}

//NewConfig - выделение памяти для новой конфигурации
func New() *Config {
	s := Config{
		DBPath:    "postgresql://postgres:admin@127.0.0.1:5432/gophermart?sslmode=disable",
		Bind:      "127.0.0.1:8080",
		AccSystem: "",
	}
	err := s.readEnv()
	if err != nil {
		log.Fatal(err)
	}
	s.readCli()
	return &s
}

//isFlagPassed - проверка применение флага
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
	// parsed := fmt.Sprintf("Evironment parsed:\nBASE_URL=%s\nSERVER_ADDRESS=%s\nFILE_STORAGE_PATH=%s\nDATABASE_DSN=%s\n", c.BaseURL, c.ServerAddress, c.FileStoragePath, c.Database)
	// log.Println(parsed)
	return nil
}

var flags = map[string]string{
	"a":     "RUN_ADDRESS",
	"d":     "DATABASE_URI",
	"r":     "ACCRUAL_SYSTEM_ADDRESS",
	"debug": "DEBUG",
	"l":     "LOG_LEVEL",
}

var path = flag.String("a", "", flags["a"])
var bind = flag.String("d", "", flags["d"])
var accPath = flag.String("r", "", flags["r"])
var debug = flag.Bool("debug", false, flags["debug"])
var level = flag.Int("l", 3, flags["l"])

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
	}
	// parsed := fmt.Sprintf("Flags parsed:\nBASE_URL=%s\nSERVER_ADDRESS=%s\nFILE_STORAGE_PATH=%s\nDATABASE_DSN=%s\n", *baseURL, *srvAddr, *filePath, *dbPath)
	// log.Println(parsed)

}
