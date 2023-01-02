package pg

import (
	"fmt"
	"strings"

	"github.com/jackc/pglogrepl"
)

type Columns map[string]Column

type Column struct {
	Data Data
	Meta MetaData
}

type Data struct {
	Type   uint8
	Length uint32
	Data   []byte
}

type MetaData struct {
	Flags    uint8
	Name     string
	Type     uint32
	Modifier int32
}

func (d Data) Changed() bool {
	if d.Type == 'u' {
		return false
	}
	return true
}

func (c Column) Sql() string {
	switch c.Data.Type {
	case 'n': // nul
		return "NULL"
	case 'u': // unchanged toast
		return "UNCHANGED"
	case 't': // text
		val, err := decodeTextColumnData(c.Data.Data, c.Meta.Type)
		if err != nil {
			log.Fatal("error decoding column data:", err)
		}
		switch v := val.(type) {
		case nil:
			return "NULL"
		case int, int32:
			return fmt.Sprintf("%d", v)
		case string:
			return stringValueSql(v)
		default:
			log.Fatalf("pgarrow does not work (yet) with values like %v.(%T)", val, val)
		}
	default:
		log.Fatalf("column data has unexpected datatype %c (instead of 'n', 'u', or 't')", c.Data.Type)
	}
	log.Fatalf("I did not expect to get to the end of Column{%v}.String().", c)
	return ""
}

func ColValsFromLogMsg(cols []*pglogrepl.TupleDataColumn, relInfo *pglogrepl.RelationMessage) (cvs Columns) {
	cvs = make(Columns)
	for idx, col := range cols {
		meta := relInfo.Columns[idx]
		name := meta.Name
		cvs[name] = Column{
			Data: Data{
				Type:   col.DataType,
				Length: col.Length,
				Data:   col.Data,
			},
			Meta: MetaData{
				Flags:    meta.Flags,
				Name:     meta.Name,
				Type:     meta.DataType,
				Modifier: meta.TypeModifier,
			},
		}
	}
	return cvs
}

func WhereFromLogMsg(cols []*pglogrepl.RelationMessageColumn, newVals Columns) Columns {
	where := make(Columns)
	for _, col := range cols {
		if col.Flags == 1 {
			where[col.Name] = newVals[col.Name]
		}
	}
	return where
}

func (cvs Columns) ColNamesValues() (names []string, values []string) {
	for name, col := range cvs {
		if col.Data.Changed() {
			names = append(names, identifierNameSql(name))
			values = append(values, col.Sql())
		}
	}
	if len(values) == 0 {
		log.Fatal("Seems we are about to run an INSERT query with an empty value list!!!")
	}
	return names, values
}

func (cvs Columns) colIsValues() []string {
	var parts []string
	for key, value := range cvs {
		// Values should already be parsed into this topic as valid SQL, like NULL, 0, 1.234 or 'whatever text with '' quotes'
		// repackValueSql is there to make sure we don't allow for SQL Injection, by
		// allowing for  values like NULL, 0, 1.234, etc. And repacking text (unquoting and re-quoting)...
		part := fmt.Sprintf("%s = %s", identifierNameSql(key), value.Sql())
		parts = append(parts, part)
	}
	return parts
}

func (cvs Columns) SetSQL() string {
	parts := cvs.colIsValues()
	if len(parts) == 0 {
		log.Fatal("Seems we are about to run an update query with an empty SET statement!!!")
	}
	return strings.Join(parts, ", ")
}

func (cvs Columns) WhereSQL() string {
	parts := cvs.colIsValues()
	if len(parts) == 0 {
		log.Fatal("Seems we are about to run a query without WHERE statement!!!")
	}
	return strings.Join(parts, " AND ")
}

// ColumnValue is an array of columns and values that we will use to create  the keylist.
// The values are stored as string because a JSON is created anyway.
type ColumnValue struct {
	Key   string
	Value string
	Type  int
}
