package pg

import (
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
)

type RelationMessages map[uint32]*pglogrepl.RelationMessage

type Conn struct {
	config           *Config
	conn             *pgconn.PgConn
	sysIdent         pglogrepl.IdentifySystemResult
	relationMessages RelationMessages
}

func NewConn(conf *Config) (c *Conn) {
	return &Conn{
		config:           conf,
		relationMessages: make(RelationMessages),
	}
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
	c.sysIdent, err = pglogrepl.IdentifySystem(ctx, c.conn)
	if err != nil {
		log.Fatalln("IdentifySystem failed:", err)
	}
	log.Info("SystemID:", c.sysIdent.SystemID, "Timeline:", c.sysIdent.Timeline, "XLogPos:", c.sysIdent.XLogPos, "DBName:", c.sysIdent.DBName)

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

//func (c *Conn) RedoTransaction(t Transaction) (err error) {
//
//	if err = c.Connect(); err != nil {
//		return err
//	}
//	cur := c.conn.ExecParams(ctx, t.ParamSQL(), t.ParamValues(), t.ParamOIDs(), nil, nil)
//	return cur.Close()
//}
