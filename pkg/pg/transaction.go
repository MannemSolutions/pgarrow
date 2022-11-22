package pg

import (
	"encoding/json"
	"fmt"
	"github.com/jackc/pglogrepl"
)

// The Transaction struct is used as a format for storing
type Transaction struct {
	LSN       pglogrepl.LSN
	Type      string
	Namespace string
	RelName   string
	Vals      ColumnValues
	Where     ColumnValues
}

func (t Transaction) Dump() ([]byte, error) {
	return json.Marshal(t)
}

func TransactionFromBytes(j []byte) (t Transaction, err error) {
	if err = json.Unmarshal(j, t); err != nil {
		return Transaction{}, err
	}
	return t, nil
}

func (t Transaction) RelationName() string {
	return fmt.Sprintf("%s.%s", identifierNameSql(t.Namespace), identifierNameSql(t.RelName))
}

func (t Transaction) Sql() string {
	var sql string
	switch t.Type {
	case "INSERT":
		sql = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			t.RelationName(),
			t.Vals.Columns(),
			t.Vals.Values())
	case "TRUNCATE":
		sql = fmt.Sprintf("TRUNCATE TABLE ONLY %s", t.RelationName())
	case "DELETE":
		sql = fmt.Sprintf("DELETE FROM %s WHERE %s", t.RelationName(), t.Where.WhereSQL())
	case "UPDATE":
		sql = fmt.Sprintf("UPDATE %s SET %s WHERE %s",
			t.RelationName(),
			t.Vals.SetSQL(),
			t.Where.WhereSQL())
	default:
		log.Fatalf("received unknown transaction type (%s)", t.Type)
	}
	return sql
}
