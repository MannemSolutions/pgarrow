package arrow

import "github.com/mannemsolutions/pgarrrow/pkg/pg"

type Config struct {
	Type       string `yaml:"type"`
	ConnParams pg.Dsn `yaml:"conn_params"`
	Role       string `yaml:"role"`
}
