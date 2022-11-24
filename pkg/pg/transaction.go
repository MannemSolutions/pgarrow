package pg

import (
	"encoding/json"
	"fmt"
	"strings"
)

var (
	ValidTypes = map[string]bool{
		"INSERT":   false,
		"UPDATE":   false,
		"DELETE":   false,
		"TRUNCATE": true,
	}
)

// The Transaction struct is used as a format for storing
type Transactions []Transaction
type Transaction struct {
	LSN    uint64
	Type   string
	Tables Tables
	Values ColumnValues
	Where  ColumnValues
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
	if !t.Validate() {
		return Transaction{}, fmt.Errorf("invalid transaction from json")
	}
	log.Debug(t)
	return t, nil
}

func (t Transaction) Validate() bool {
	if multipleTables, ok := ValidTypes[t.Type]; !ok {
		log.Debugf("invalid transaction type %s", t.Type)
		return false
	} else if !multipleTables && len(t.Tables) > 1 {
		log.Debugf("transaction type %s does not support multiple tables", t.Type)
		return false
	} else if len(t.Tables) == 0 {
		log.Debugln("table needs to be set")
		return false
	}
	return true
}

func (t Transaction) Sql() string {
	var sql string
	if !t.Validate() {
		return ""
	}
	switch t.Type {
	case "INSERT":
		names, values := t.Values.ColNamesValues()
		sql = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			t.Tables[0].RelationName(),
			strings.Join(names, ","),
			strings.Join(values, ","))
	case "TRUNCATE":
		sql = fmt.Sprintf("TRUNCATE TABLE ONLY %s", t.Tables.RelationNames())
	case "DELETE":
		sql = fmt.Sprintf("DELETE FROM %s WHERE %s", t.Tables[0].RelationName(), t.Where.WhereSQL())
	case "UPDATE":
		sql = fmt.Sprintf("UPDATE %s SET %s WHERE %s",
			t.Tables[0].RelationName(),
			t.Values.SetSQL(),
			t.Where.WhereSQL())
	default:
		log.Errorf("received unknown transaction type (%s)", t.Type)
	}
	return sql
}
