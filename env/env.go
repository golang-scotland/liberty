package env

import (
	"errors"
	"os"
)

const (
	Dev = iota
	Test
	Stage
	Prod
)

type Env int

var (
	current         Env
	ErrInvalidValue = errors.New("env: unknown or invalid value")
	names           = map[Env]string{
		Dev:   "dev",
		Test:  "test",
		Stage: "stage",
		Prod:  "prod",
	}
)

func (e Env) String() string {
	return names[e]
}

// Get the current env var 'APP_ENV' value or default to Dev
func Get() Env {
	appEnv := os.Getenv("APP_ENV")
	switch appEnv {
	case "prod":
		return Prod
	case "stage":
		return Stage
	case "dev", "":
		return Dev
	default:
		return Dev
	}
}

func Set(e Env) error {
	switch e {
	case Dev, Test, Stage, Prod:
		current = e
	default:
		return ErrInvalidValue
	}
	return nil
}

func Hostname() (host string) {
	var err error
	if host, err = os.Hostname(); err == nil {
		return host
	}

	panic(err.Error())
}
