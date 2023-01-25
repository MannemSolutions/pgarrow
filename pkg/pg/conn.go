package pg

import (
	"fmt"
	"go.uber.org/zap"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
)

type RelationMessages map[uint32]*pglogrepl.RelationMessage

type Conn struct {
	config                      *Config
	rConn                       *pgconn.PgConn
	qConn                       *pgconn.PgConn
	relationMessages            RelationMessages
	XLogPos                     pglogrepl.LSN
	lastPrimaryKeepaliveMessage time.Time
	outOfSync                   bool
}

func NewConn(conf *Config) (c *Conn) {
	return &Conn{
		config:                      conf,
		relationMessages:            make(RelationMessages),
		lastPrimaryKeepaliveMessage: time.Now(),
	}
}

func (c Conn) Clone() (new *Conn) {
	newConfig := c.config.Clone()
	return NewConn(&newConfig)
}

func (c *Conn) Connect() (err error) {
	if c.rConn != nil {
		if c.rConn.IsClosed() {
			c.rConn = nil
		} else {
			return nil
		}
	}
	for {
		c.rConn, err = pgconn.Connect(ctx, c.config.DSN.ConnString(true))
		if err == nil {
			break
		}
		log.Errorln("Cannot connect to Postgres:", err.Error())
		log.Infof("Retrying in 10 seconds")
		time.Sleep(10 * time.Second)
	}
	if err = c.getPgTypes(); err != nil {
		return err
	}
	log.Debugln("successfully connected to postgres")
	return nil
}

func (c *Conn) qryConnect() (err error) {
	if c.qConn != nil {
		if c.qConn.IsClosed() {
			c.qConn = nil
		} else {
			return nil
		}
	}
	c.qConn, err = pgconn.Connect(ctx, c.config.DSN.ConnString(false))
	if err != nil {
		c.qConn = nil
		return err
	}

	return nil
}

func (c *Conn) MustClose() {
	if err := c.Close(); err != nil {
		log.Fatalf("Error closing pg connection: %e", err)
	}
}

func (c *Conn) Close() (err error) {
	if err = _close(c.rConn); err != nil {
		log.Infof("Error closing rConn")
		return err
	}
	c.rConn = nil
	if err = _close(c.qConn); err != nil {
		log.Infof("Error closing qConn")
		return err
	}
	c.qConn = nil
	log.Debugln("connection successfully closed")
	return nil
}

func _close(c *pgconn.PgConn) (err error) {
	if c == nil {
		return nil
	}
	if c.IsClosed() {
		return nil
	}
	if err = c.Close(ctx); err != nil {
		return err
	}
	return nil
}

func (c *Conn) RunSQL(sql string) (err error) {
	if err = c.Connect(); err != nil {
		return err
	}
	log.Debugf("Running SQL: %s", sql)
	cur := c.rConn.Exec(ctx, sql)
	if err = cur.Close(); err == nil {
		return nil
	} else if pgErr, ok := err.(*pgconn.PgError); !ok {
		log.Error("Unexpected error while running query: (%T)->%v", err, err)
		return err
	} else if pgErr.Code != "57P01" {
		log.Error("unexpected Postgres error while running query: (%T)->%v", pgErr, pgErr)
		return pgErr
	} else if closeErr := c.Close(); closeErr != nil {
		log.Error("tried to resolve SQLSTATE 57P01 while running query, "+
			"but closing connection failed: (%T)->%v", closeErr, closeErr)
		return closeErr
	}
	log.Info("recovering from SQLSTATE 57P01, rerunning query")
	if runErr := c.RunSQL(sql); runErr != nil {
		log.Error("tried to resolve SQLSTATE 57P01 while running query, "+
			"but rerunning the query failed: (%T)->%v", runErr, runErr)
		return runErr
	}
	return nil
}

func MustCloseResult(result *pgconn.ResultReader) {
	if _, err := result.Close(); err != nil {
		log.Fatalf("Error while closing result: %e", err)
	}
}

func (c *Conn) GetRows(query string) (answer []map[string]string, err error) {
	if err = c.qryConnect(); err != nil {
		return nil, err
	}
	log.Debugf("Running SQL: %s", query)
	cur := c.qConn.Exec(ctx, query)
	if next := cur.NextResult(); !next {
		return nil, fmt.Errorf("query did not return results: %s", query)
	}
	result := cur.ResultReader()
	defer MustCloseResult(result)
	var hdr []string
	for _, col := range result.FieldDescriptions() {
		hdr = append(hdr, col.Name)
	}
	for result.NextRow() {
		row := make(map[string]string)
		for i, f := range result.Values() {
			row[hdr[i]] = string(f)
		}
		answer = append(answer, row)
	}
	return answer, cur.Close()
}

func (c *Conn) getSlotInfo() (slotInfos, error) {
	if results, err := c.GetRows("select slot_name, active, restart_lsn from pg_replication_slots"); err != nil {
		return slotInfos{}, err
	} else {
		sis := make(slotInfos)
		for _, result := range results {
			if name, ok := result["slot_name"]; !ok {
				return slotInfos{}, fmt.Errorf("query results misses `slot_name` field")
			} else if active, ok := result["active"]; !ok {
				return slotInfos{}, fmt.Errorf("query results misses `active` field")
			} else if bActive, err := strconv.ParseBool(active); err != nil {
				return slotInfos{}, err
			} else if restart, ok := result["restart_lsn"]; !ok {
				return slotInfos{}, fmt.Errorf("query results misses `restart_lsn` field")
			} else if restartLsn, err := pglogrepl.ParseLSN(restart); err != nil {
				return slotInfos{}, err
			} else {
				log.Debugf("Slot: %s, active: %s, restartLSN: %s", name, active, restartLsn)
				sis[name] = slotInfo{
					name:       name,
					active:     bActive,
					restartLsn: restartLsn,
				}
			}
		}
		return sis, nil
	}
}

func (c *Conn) getPgTypes() error {
	if answer, queryErr := c.GetRows("select oid, typname, typcategory from pg_type"); queryErr != nil {
		return queryErr
	} else {
		oidToPgType = make(map[uint32]string)
		oidToPgCategory = make(map[uint32]uint8)
		for _, row := range answer {
			if sOid, ok := row["oid"]; !ok {
				return fmt.Errorf("unexpected result, expected oid column")
			} else if oid, convErr := strconv.ParseInt(sOid, 10, 32); convErr != nil {
				log.Errorf("Error while converting string %s to int: %e", row["oid"], convErr)
			} else if name, ok := row["typname"]; !ok {
				return fmt.Errorf("unexpected result, expected typname column")
			} else if cat, ok := row["typcategory"]; !ok {
				return fmt.Errorf("unexpected result, expected typcategory column")
			} else if len(cat) != 1 {
				return fmt.Errorf("unexpected result, expected typcategory to be 1 character only")
			} else {
				oidToPgType[uint32(oid)] = name
				oidToPgCategory[uint32(oid)] = uint8([]rune(cat)[0])
			}
		}
	}
	return nil
}

func (c *Conn) GetXLogPos() (pglogrepl.LSN, error) {
	if slots, err := c.getSlotInfo(); err != nil {
		return 0, err
	} else if slot, ok := slots[c.config.Slot]; !ok {
		return 0, fmt.Errorf("could not find slot info for this slot")
	} else if slot.active {
		return 0, fmt.Errorf("slot %s is already active", slot.name)
	} else {
		c.XLogPos = slot.restartLsn
		log.Debugf("restart LSN for slot %s: %d", slot.name, c.XLogPos)
	}
	return c.XLogPos, nil
}
func (c *Conn) GetTableFromOID(oid uint32) (t Table, err error) {
	tmpConn := c.Clone()
	defer tmpConn.MustClose()
	qry := "select pg_namespace.nspname namespace, pg_class.relname table_name " +
		"from pg_class " +
		"inner join pg_namespace " +
		"on pg_class.relnamespace = pg_namespace.oid " +
		"where pg_class.oid = %d"
	qry = fmt.Sprintf(qry, oid)
	var results []map[string]string
	if results, err = c.GetRows(qry); err != nil {
		return t, err
	} else if len(results) == 0 {
		log.Fatalf("table with oid %d does not exist in this database", oid)
	} else if len(results) > 1 {
		log.Fatalf("multiple tables with oid %d", oid)
	}
	result := results[0]
	if namespace, ok := result["namespace"]; !ok {
		log.Fatal("unexpected result (namespace column missing)")
	} else if tableName, ok := result["table_name"]; !ok {
		log.Fatal("unexpected result (tablename column missing)")
	} else {
		t.Namespace = namespace
		t.TableName = tableName
		log.Debugf("namespace %s, table_name %s", namespace, tableName)
	}
	return t, nil
}

func (c *Conn) getPrimaryKeyCols(t Table) (cols []string, err error) {
	if rows, err := c.GetRows(t.PrimaryKeyQuery()); err != nil {
		return nil, err
	} else {
		for _, row := range rows {
			if colName, ok := row["column_name"]; !ok {
				return nil, fmt.Errorf("could not find column `column_name` in primary key query")
			} else {
				cols = append(cols, colName)
			}
		}
	}
	return cols, nil
}

func (c *Conn) ProcessMsg(msg []byte) (err error) {
	if ce := quickLog.Check(zap.DebugLevel, "Processing messages"); ce != nil {
		ce.Write(
			zap.Int("length", len(msg)),
		)
	}
	var t Transaction
	if t, err = TransactionFromBytes(msg); err != nil {
		return err
	}
	sql := t.Sql()
	if err = c.RunSQL(sql); err == nil {
		log.Debugf("succesfully ran %s", sql)
	} else if pgErr, ok := err.(*pgconn.PgError); !ok {
		return err
	} else if description, exists := c.config.SkipErrors[pgErr.Code]; exists {
		log.Debugf("skipping error code %s (%s)", pgErr.Code, description)
		return nil
	} else {
		log.Error(pgErr)
		if ce := quickLog.Check(zap.DebugLevel, "to skip, add this to config"); ce != nil {
			ce.Write(
				zap.String("key", fmt.Sprintf("pg_config.skip_errors.%s", pgErr.Code)),
				zap.String("value", strings.Replace(pgErr.Message, "\"", "'", -1)),
			)
		}

		return pgErr
	}
	return nil

}

func (c *Conn) StreamTables(PostProcessor func([]byte) error) (err error) {
	if !c.outOfSync {
		return nil
	}
	if answer, queryErr := c.GetRows("select schemaname, tablename from pg_publication_tables"); queryErr != nil {
		log.Errorf("error while retrieving tables to stream: %e", queryErr)
		return queryErr
	} else {
		for _, row := range answer {
			if schemaName, sOk := row["schemaname"]; !sOk {
				err = fmt.Errorf("pg_publication_tables result has no column `schemaname`")
				log.Errorf("%e", err)
				return err
			} else if tableName, tOk := row["tablename"]; !tOk {
				err = fmt.Errorf("pg_publication_tables result has no column `tablename`")
				log.Errorf("%e", err)
				return err
			} else {
				table := Table{Namespace: schemaName, TableName: tableName}
				if err = c.StreamTable(PostProcessor, table); err != nil {
					log.Errorf("error while streaming table %s: %e", table, queryErr)
					return queryErr
				}
			}
		}
	}
	c.outOfSync = false
	return nil

}

func (c *Conn) StreamTable(PostProcessor func([]byte) error, table Table) (err error) {
	if err = c.qryConnect(); err != nil {
		return err
	}
	log.Debugf("Getting all records from: %s", table.RelationName())
	cur := c.qConn.Exec(ctx, table.SelectAllQuery())
	result := cur.ResultReader()
	defer MustCloseResult(result)
	values := make(Columns)
	var hdr []string
	for _, fd := range result.FieldDescriptions() {
		hdr = append(hdr, fd.Name)
		values[fd.Name] = Column{
			Meta: MetaData{
				Name:     fd.Name,
				TypeOID:  fd.DataTypeOID,
				TypeName: oidToPgType[fd.DataTypeOID],
				Modifier: fd.TypeModifier,
			},
		}
	}
	where := make(Columns)
	var pkcs []string
	if pkcs, err = c.getPrimaryKeyCols(table); err != nil {
		return err
	}
	for result.NextRow() {
		values = values.Clone()
		for i, f := range result.Values() {
			value := values[hdr[i]]
			value.Data = Data{
				Type:   oidToPgCategory[value.Meta.TypeOID],
				Length: uint32(len(f)),
				Data:   f,
			}
			values[hdr[i]] = value
		}
		for _, pkc := range pkcs {
			where[pkc] = values[pkc]
		}
		t := Transaction{
			LSN:    1,
			Type:   "UPSERT",
			Tables: Tables{table},
			Values: values,
			Where:  where,
		}
		var raw []byte
		if raw, err = t.Dump(); err != nil {
			log.Errorf("Could not convert row from %s to json: %e", table.RelationName(), err)
			return err
		} else if err = PostProcessor(raw); err != nil {
			log.Errorf("Could not postprocess json row from %s: %e", table.RelationName(), err)
			return err
		}
	}
	return cur.Close()
}
