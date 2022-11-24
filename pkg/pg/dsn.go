package pg

import (
	"fmt"
	"strings"
)

type Dsn map[string]string

func (d Dsn) String(masked bool) string {
	var parts []string
	for k, v := range d {
		if k == "password" {
			v = "*****"
		}
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", k, strings.Replace(v, "\"", "\"\"", -1)))
	}
	return strings.Join(parts, " ")
}
func (d Dsn) ConnString() (dsn string) {
	var pairs []string
	for key, value := range d {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, connectStringValue(value)))
	}
	return strings.Join(pairs[:], " ")
}

func (d Dsn) Clone() (new Dsn) {
	new = make(Dsn)
	for k, v := range d {
		new[k] = v
	}
	return new
}
