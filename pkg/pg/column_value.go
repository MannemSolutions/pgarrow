package pg

import (
	"fmt"
	"strings"

	"github.com/jackc/pglogrepl"
)

type ColumnValues map[string]string

func ColValsFromLogMsg(cols []*pglogrepl.TupleDataColumn, relInfo *pglogrepl.RelationMessage) (cvs ColumnValues) {
	cvs = make(ColumnValues)
	for idx, col := range cols {
		colName := relInfo.Columns[idx].Name
		switch col.DataType {
		case 'n': // null
			cvs[colName] = "NULL"
		case 'u': // unchanged toast
			// This TOAST value was not changed. TOAST values are not stored in the tuple, and logical replication doesn't want to spend a disk read to fetch its value for you.
		case 't': //text
			val, err := decodeTextColumnData(col.Data, relInfo.Columns[idx].DataType)
			if err != nil {
				log.Fatalln("error decoding column data:", err)
			}

			switch v := val.(type) {
			case nil:
				cvs[colName] = "NULL"
			case int, int32:
				cvs[colName] = fmt.Sprintf("%d", v)
			case string:
				cvs[colName] = stringValueSql(v)
			default:
				log.Fatalf("pgarrow does not work (yet) with values like %v.(%T)", val, val)
			}
		}
	}
	return cvs
}

func WhereFromLogMsg(cols []*pglogrepl.RelationMessageColumn, newVals ColumnValues) ColumnValues {
	where := make(ColumnValues)
	for _, col := range cols {
		if col.Flags == 1 {
			where[col.Name] = newVals[col.Name]
		}
	}
	return where
}

func (ckvs ColumnValues) ColNamesValues() (names []string, values []string) {
	for name, value := range ckvs {
		names = append(names, identifierNameSql(name))
		values = append(values, repackValueSql(value))
	}
	if len(values) == 0 {
		log.Fatal("Seems we are about to run an INSERT query with an empty value list!!!")
	}
	return names, values
}

func (ckvs ColumnValues) colIsValues() []string {
	var parts []string
	for key, value := range ckvs {
		// Values should already be parsed into this topic as valid SQL, like NULL, 0, 1.234 or 'whatever text with '' quotes'
		// repackValueSql is there to make sure we don't allow for SQL Injection, by
		// allowing for  values like NULL, 0, 1.234, etc. And repacking text (unquoting and re-quoting)...
		part := fmt.Sprintf("%s = %s", identifierNameSql(key), repackValueSql(value))
		parts = append(parts, part)
	}
	return parts
}

func (ckvs ColumnValues) SetSQL() string {
	parts := ckvs.colIsValues()
	if len(parts) == 0 {
		log.Fatal("Seems we are about to run an update query with an empty SET statement!!!")
	}
	return fmt.Sprintf("%s", strings.Join(parts, ", "))
}

func (ckvs ColumnValues) WhereSQL() string {
	parts := ckvs.colIsValues()
	if len(parts) == 0 {
		log.Fatal("Seems we are about to run a query without WHERE statement!!!")
	}
	return fmt.Sprintf("%s", strings.Join(parts, " AND "))
}

// ColumnValue is an array of columns and values that we will use to create  the keylist.
// The values are stored as string because a JSON is created anyway.
type ColumnValue struct {
	Key   string
	Value string
	Type  int
}
