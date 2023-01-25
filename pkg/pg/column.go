package pg

import (
	"database/sql/driver"
	"fmt"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgtype"
	"strings"
	"time"
)

type Columns map[string]Column

func ColValsFromLogMsg(cols []*pglogrepl.TupleDataColumn, relInfo *pglogrepl.RelationMessage) (cvs Columns) {
	cvs = make(Columns)
	for idx, col := range cols {
		meta := relInfo.Columns[idx]
		name := meta.Name
		if typeName, ok := oidToPgType[meta.DataType]; !ok {
			log.Fatalf("trying to get Type Name for unknown value type (oid %d)", meta.DataType)
		} else {
			cvs[name] = Column{
				Data: Data{
					Type:   col.DataType,
					Length: col.Length,
					Data:   col.Data,
				},
				Meta: MetaData{
					Flags:    meta.Flags,
					Name:     meta.Name,
					TypeOID:  meta.DataType,
					TypeName: typeName,
					Modifier: meta.TypeModifier,
				},
			}
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

func (cvs Columns) Clone() (new Columns) {
	for name, val := range cvs {
		new[name] = Column{
			Data: val.Data.Clone(),
			Meta: val.Meta.Clone(),
		}
	}
	return new
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

type Column struct {
	Data Data
	Meta MetaData
}

func tryValuerToString(v driver.Valuer) (string, error) {
	if nVal, err := v.Value(); err != nil {
		return "", fmt.Errorf("error converting %v to driver.Value: %e", v, err)
	} else if sVal, ok := nVal.(string); !ok {
		return "", fmt.Errorf("expected driver.Value %e (%T) to be string", v, v)
	} else {
		return stringValueSql(sVal), nil
	}
}

func (c Column) Sql() string {
	switch c.Data.Type {
	case 'n': // null
		return "NULL"
	case 'u': // unchanged toast
		log.Panicf("trying to get SQL from unchanged TOAST value, this should never happen!!!")
	case 't': // text
		switch c.Meta.TypeName {
		case "json", "jsonb", "xml":
			return fmt.Sprintf("%s::%s", stringValueSql(string(c.Data.Data)), c.Meta.TypeName)
		default:
			//xml is a special case which is not implemented in decodeTextColumnData
			//we can manage it separately
			if c.Meta.TypeName == "xml" {
				return fmt.Sprintf("%s::%s", stringValueSql(string(c.Data.Data)), c.Meta.TypeName)
			}
			val, err := decodeTextColumnData(c.Data.Data, c.Meta.TypeOID)
			if err != nil {
				log.Fatal("error decoding column data:", err)
			}
			switch v := val.(type) {
			case nil:
				return "NULL"
			case bool:
				return fmt.Sprintf("%v", v)
			case time.Time:
				return fmt.Sprintf("'%s'::%s", v.Format("2006-01-02T15:04:05.000Z"), c.Meta.TypeName)
			case pgtype.Interval:
				return fmt.Sprintf("'%s'::%s",
					fmt.Sprintf("%d days %d months %d microseconds", v.Days, v.Months, v.Microseconds),
					c.Meta.TypeName)
			case int, int8, int16, int32, int64:
				return fmt.Sprintf("%d", v)
			case uint, uint8, uint16, uint32, uint64:
				return fmt.Sprintf("%d", v)
			case float32, float64:
				return fmt.Sprintf("%f", v)
			case string:
				return fmt.Sprintf("%s::%s", stringValueSql(v), c.Meta.TypeName)
			case driver.Valuer:
				if sVal, err := tryValuerToString(v); err == nil {
					return fmt.Sprintf("%s::%s", sVal, c.Meta.TypeName)
				} else if stringer, ok := v.(fmt.Stringer); ok {
					return fmt.Sprintf("%s::%s", stringValueSql(stringer.String()), c.Meta.TypeName)
				}
			case fmt.Stringer:
				return fmt.Sprintf("%s::%s", stringValueSql(v.String()), c.Meta.TypeName)
			case []byte:
				return fmt.Sprintf("%s::%s", stringValueSql(string(c.Data.Data)), c.Meta.TypeName)
			default:
				log.Fatalf("pgarrow does not work (yet) with (pg=>%s,go=>%T) values", c.Meta.TypeName, v)
			}
		}
	default:
		log.Fatalf("column data has unexpected datatype %c (instead of 'n', 'u', or 't')", c.Data.Type)
	}
	log.Fatalf("I did not expect to get to the end of Column{%v}.String().", c)
	return ""
}

type Data struct {
	Type   uint8
	Length uint32
	Data   []byte
}

func (d Data) Clone() Data {
	return d
}

func (d Data) Changed() bool {
	return d.Type != 'u'
}

type MetaData struct {
	Flags    uint8
	Name     string
	TypeOID  uint32
	TypeName string
	Modifier int32
}

func (m MetaData) Clone() MetaData {
	return m
}
