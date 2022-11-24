package pg

import (
	"fmt"
	"strings"
)

type Tables []Table
type Table struct {
	Namespace string
	TableName string
}

func (ts Tables) RelationNames() string {
	var names []string
	for _, t := range ts {
		names = append(names, t.RelationName())
	}
	return strings.Join(names, ",")
}

func (t Table) RelationName() string {
	return fmt.Sprintf("%s.%s", identifierNameSql(t.Namespace), identifierNameSql(t.TableName))
}
