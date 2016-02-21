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
		return Stage
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
