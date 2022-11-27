package pg

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	reSimpleSQLValue *regexp.Regexp
)

// identifierNameSql returns the object name ready to be used in a sql query as an object name (e.a. Select * from %s)
func identifierNameSql(objectName string) (escaped string) {
	return fmt.Sprintf("\"%s\"", strings.Replace(objectName, "\"", "\"\"", -1))
}

// stringValueSql uses proper quoting for values in SQL queries
func stringValueSql(stringValue string) (escaped string) {
	return fmt.Sprintf("'%s'", strings.Replace(stringValue, "'", "''", -1))
}

// repackValueSql unpacks and repacks, just to be sure we don't introduce SQL injection
func repackValueSql(sqlValue string) (repacked string) {
	if reSimpleSQLValue == nil {
		reSimpleSQLValue = regexp.MustCompile(`^(NULL|[\d.]+)$`)
	}
	if reSimpleSQLValue.Match([]byte(sqlValue)) {
		return sqlValue
	}
	if !(strings.HasPrefix(sqlValue, "'") && strings.HasSuffix(sqlValue, "'")) {
		log.Fatalf("This does not seem like a valid SQL value: %s", sqlValue)
	}
	// Unpacking and repacking string just to be sure!!!
	return stringValueSql(strings.Replace(sqlValue[1:len(sqlValue)-1], "''", "'", -1))
}

//func decodeTextColumnData(data []byte, dataType uint32) (interface{}, error) {
//	var decoder pgtype.TextDecoder
//	if connInfo == nil {
//		connInfo = pgtype.NewConnInfo()
//	}
//	if dt, ok := connInfo.DataTypeForOID(dataType); ok {
//		decoder, ok = dt.Value.(pgtype.TextDecoder)
//		if !ok {
//			decoder = &pgtype.GenericText{}
//		}
//	} else {
//		decoder = &pgtype.GenericText{}
//	}
//	if err := decoder.DecodeText(connInfo, data); err != nil {
//		return nil, err
//	}
//	return decoder.(pgtype.Value).Get(), nil
//}

func decodeTextColumnData(data []byte, dataType uint32) (interface{}, error) {
	if typeMap == nil {
		typeMap = pgtype.NewMap()
	}
	if dt, ok := typeMap.TypeForOID(dataType); ok {
		return dt.Codec.DecodeValue(typeMap, dataType, pgtype.TextFormatCode, data)
	}
	return string(data), nil
}

// connectStringValue uses proper quoting for connect string values
func connectStringValue(objectName string) (escaped string) {
	return fmt.Sprintf("'%s'", strings.Replace(objectName, "'", "\\'", -1))
}
