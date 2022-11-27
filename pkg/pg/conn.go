package pg

import (
	"fmt"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"strconv"
)

type RelationMessages map[uint32]*pglogrepl.RelationMessage

type Conn struct {
	config           *Config
	rConn            *pgconn.PgConn
	qConn            *pgconn.PgConn
	relationMessages RelationMessages
	XLogPos          pglogrepl.LSN
}

func NewConn(conf *Config) (c *Conn) {
	return &Conn{
		config:           conf,
		relationMessages: make(RelationMessages),
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
	c.rConn, err = pgconn.Connect(ctx, c.config.DSN.ConnString(true))
	if err != nil {
		c.rConn = nil
		return err
	}

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
	return cur.Close()
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
	if _, err = result.Close(); err != nil {
		return nil, err
	}
	return answer, cur.Close()
}

func (c *Conn) getSlotInfo() (sis slotInfos, err error) {
	sis = make(slotInfos)
	results, err := c.GetRows("select slot_name, active, restart_lsn from pg_replication_slots")
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

func (c Conn) ProcessMsg(msg []byte) (err error) {
	log.Debug("Processing messages")
	log.Debugf("Processing msg (%d bytes)", len(msg))
	var t Transaction
	if t, err = TransactionFromBytes(msg); err != nil {
		return err
	}
	sql := t.Sql()
	if err = c.RunSQL(sql); err == nil {
		log.Debugf("succesfully ran %s", sql)
	} else if pgErr, ok := err.(*pgconn.PgError); !ok {
		return err
	} else if pgErr.Code == "23505" {
		return pgErr
	} else {
		return pgErr
	}
	return nil

}
