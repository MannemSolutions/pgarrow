package pg

import (
	"encoding/json"
	"fmt"
	"strings"
)

// The Transaction struct is used as a format for storing
type Transaction struct {
	LSN       uint64
	Type      string
	Namespace string
	RelName   string
	Values    ColumnValues
	Where     ColumnValues
}

func (t Transaction) Dump() ([]byte, error) {
	return json.Marshal(t)
}

func TransactionFromBytes(j []byte) (t Transaction, err error) {
	log.Debugf(string(j))
	if err = json.Unmarshal(j, &t); err != nil {
		log.Debug(err)
		return Transaction{}, err
	}
	log.Debug(t)
	return t, nil
}

func (t Transaction) RelationName() string {
	return fmt.Sprintf("%s.%s", identifierNameSql(t.Namespace), identifierNameSql(t.RelName))
}

func (t Transaction) Sql() string {
	var sql string
	switch t.Type {
	case "INSERT":
		names, values := t.Values.ColNamesValues()
		sql = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			t.RelationName(),
			strings.Join(names, ","),
			strings.Join(values, ","))
	case "TRUNCATE":
		sql = fmt.Sprintf("TRUNCATE TABLE ONLY %s", t.RelationName())
	case "DELETE":
		sql = fmt.Sprintf("DELETE FROM %s WHERE %s", t.RelationName(), t.Where.WhereSQL())
	case "UPDATE":
		sql = fmt.Sprintf("UPDATE %s SET %s WHERE %s",
			t.RelationName(),
			t.Values.SetSQL(),
			t.Where.WhereSQL())
	default:
		log.Errorf("received unknown transaction type (%s)", t.Type)
	}
	return sql
}
