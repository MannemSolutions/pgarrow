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

func (t Table) SelectAllQuery() string {
	return fmt.Sprintf("SELECT * from %s.%s", identifierNameSql(t.Namespace), identifierNameSql(t.TableName))
}

func (t Table) PrimaryKeyQuery() string {
	return fmt.Sprintf(
		"select t.relname as table_name, i.relname as index_name, a.attname as column_name "+
			"from pg_namespace n, pg_class t, pg_class i, pg_index ix, pg_attribute a "+
			"where t.oid = ix.indrelid and ix.indisprimary and i.oid = ix.indexrelid and a.attrelid = t.oid "+
			"and a.attnum = ANY(ix.indkey) and t.relkind = 'r' and t.relnamespace = n.oid and n.nspname = %s "+
			"and t.relname = %s", stringValueSql(t.Namespace), stringValueSql(t.TableName))
}
