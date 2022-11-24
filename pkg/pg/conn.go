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
	conn             *pgconn.PgConn
	relationMessages RelationMessages
	XLogPos          pglogrepl.LSN
	//sysIdent         pglogrepl.IdentifySystemResult
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
	if c.conn != nil {
		if c.conn.IsClosed() {
			c.conn = nil
		} else {
			return nil
		}
	}
	c.conn, err = pgconn.Connect(ctx, c.config.DSN.ConnString())
	if err != nil {
		c.conn = nil
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
	if c.conn == nil {
		return nil
	}
	if c.conn.IsClosed() {
		c.conn = nil
		return nil
	}
	if err = c.conn.Close(ctx); err != nil {
		return err
	}
	c.conn = nil
	return nil
}

func (c *Conn) RunSQL(sql string) (err error) {
	if err = c.Connect(); err != nil {
		return err
	}
	log.Debugf("Running SQL: %s", sql)
	cur := c.conn.Exec(ctx, sql)
	return cur.Close()
}

func (c *Conn) GetRows(query string) (answer []map[string]string, err error) {
	if err = c.Connect(); err != nil {
		return nil, err
	}
	log.Debugf("Running SQL: %s", query)
	cur := c.conn.Exec(ctx, query)
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

func (c *Conn) GetSlotInfo() (sis slotInfos, err error) {
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
	//tmpConn := c.Clone()
	//defer tmpConn.MustClose()
	//delete(tmpConn.config.DSN, "replication")
	if slots, err := c.GetSlotInfo(); err != nil {
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
