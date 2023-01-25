package pg

/*
   Software is based on the pglogrepl_demo that is provided in the
	 git repository from Jack Christensen (https://github.com/jackc/pglogrepl).
*/

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
)

func (c *Conn) StartRepl() (err error) {
	if c.rConn != nil {
		log.Debugln("rConn already initialized. Assuming we are already replicating")
		return nil
	}
	if err = c.Connect(); err != nil {
		return err
	}

	_, err = pglogrepl.CreateReplicationSlot(context.Background(), c.rConn, c.config.Slot, "pgoutput",
		pglogrepl.CreateReplicationSlotOptions{})
	if pgErr, ok := err.(*pgconn.PgError); ok {
		if pgErr.Code == "42710" {
			log.Infoln("Slot already exists")
		} else {
			log.Fatalln("CreateReplicationSlot failed:", err)
		}
	} else if err != nil {
		log.Fatalln("CreateReplicationSlot failed:", err)
	} else {
		log.Info("Created temporary replication slot:", c.config.Slot)
		c.outOfSync = true
	}
	if _, err = c.GetXLogPos(); err != nil {
		return err
	}
	err = pglogrepl.StartReplication(
		context.Background(),
		c.rConn,
		c.config.Slot,
		c.XLogPos,
		pglogrepl.StartReplicationOptions{
			PluginArgs: []string{"proto_version '1'", "publication_names 'pgarrow'"}})
	if err != nil {
		log.Fatal("StartReplication failed:", err)
	}
	log.Info("Logical replication started on slot", c.config.Slot)
	return nil
}

// NextTransactions reads the next transaction and returns. For TRUNCATE this could be more than one.
func (c *Conn) NextTransactions() (t Transaction, err error) {
	standbyMessageTimeout := c.config.StandbyMessageTimeout
	nextStandbyMessageDeadline := time.Now().Add(standbyMessageTimeout)

	var (
		rawMsg       pgproto3.BackendMessage
		pkm          pglogrepl.PrimaryKeepaliveMessage
		xld          pglogrepl.XLogData
		parsedMsg    pglogrepl.Message
		relationInfo *pglogrepl.RelationMessage
	)
	for {
		if time.Now().After(nextStandbyMessageDeadline) {
			err = pglogrepl.SendStandbyStatusUpdate(context.Background(), c.rConn, pglogrepl.StandbyStatusUpdate{WALWritePosition: c.XLogPos})
			if err != nil {
				log.Fatal("SendStandbyStatusUpdate failed:", err)
			}
			//log.Debug("Sent Standby status message")
			nextStandbyMessageDeadline = time.Now().Add(standbyMessageTimeout)
		}

		ctxDeadline, cancel := context.WithDeadline(context.Background(), nextStandbyMessageDeadline)
		rawMsg, err = c.rConn.ReceiveMessage(ctxDeadline)
		cancel()
		if err != nil {
			if pgconn.Timeout(err) {
				continue
			}
			log.Info("ReceiveMessage failed:", err)
			return t, err
		}

		if errMsg, ok := rawMsg.(*pgproto3.ErrorResponse); ok {
			log.Error("received Postgres WAL error: ", errMsg)
			return t, err
		}

		msg, ok := rawMsg.(*pgproto3.CopyData)
		if !ok {
			err = fmt.Errorf("Received unexpected message: %T, %v", rawMsg, msg)
			log.Error(err)
			return t, err
		}

		switch msg.Data[0] {
		case pglogrepl.PrimaryKeepaliveMessageByteID:
			pkm, err = pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
			if err != nil {
				log.Fatal("ParsePrimaryKeepaliveMessage failed:", err)
			}
			// This makes sure that the debug log is not flooded when Postgres stops
			if ce := quickLog.Check(zap.DebugLevel, "Primary Keepalive Message"); ce != nil {
				if time.Since(c.lastPrimaryKeepaliveMessage) > time.Second {
					// If debug-level log output isn't enabled or if zap's sampling would have
					// dropped this log entry, we don't allocate the slice that holds these
					// fields.
					ce.Write(zap.Any("data", pkm))
					c.lastPrimaryKeepaliveMessage = time.Now()
				}
			}

			if pkm.ReplyRequested {
				nextStandbyMessageDeadline = time.Time{}
			}

		case pglogrepl.XLogDataByteID:
			xld, err = pglogrepl.ParseXLogData(msg.Data[1:])
			if err != nil {
				log.Fatal("ParseXLogData failed:", err)
			}
			if ce := quickLog.Check(zap.DebugLevel, "XLogData"); ce != nil {
				ce.Write(
					zap.Uint64("WALStart", uint64(xld.WALStart)),
					zap.Uint64("ServerWALEnd", uint64(xld.ServerWALEnd)),
					zap.Time("ServerTime", xld.ServerTime),
					zap.Any("WALData", xld.WALData),
				)
			}
			parsedMsg, err = pglogrepl.Parse(xld.WALData)
			if err != nil {
				log.Fatalf("Parse logical replication message: %s", err)
			}

			if ce := quickLog.Check(zap.DebugLevel, "XLogMsg"); ce != nil {
				ce.Write(zap.Any("type", parsedMsg.Type()))
			}

			switch logicalMsg := parsedMsg.(type) {
			case *pglogrepl.RelationMessage:
				c.relationMessages[logicalMsg.RelationID] = logicalMsg

			case *pglogrepl.BeginMessage:
				// Indicates the beginning of a group of changes in a transaction.
				// This is only sent for committed transactions.
				// You won't get any events from rolled back transactions.

			case *pglogrepl.CommitMessage:

			case *pglogrepl.InsertMessage:
				relationInfo, ok = c.relationMessages[logicalMsg.RelationID]
				if !ok {
					log.Fatalf("unknown relation ID %d", logicalMsg.RelationID)
				}

				newValues := ColValsFromLogMsg(logicalMsg.Tuple.Columns, relationInfo)
				log.Debugf("INSERT INTO %s.%s: %v", relationInfo.Namespace, relationInfo.RelationName, relationInfo)

				t = Transaction{
					LSN:  uint64(xld.WALStart),
					Type: "INSERT",
					Tables: Tables{Table{
						Namespace: relationInfo.Namespace,
						TableName: relationInfo.RelationName,
					}},
					Values: newValues,
				}
				if ce := quickLog.Check(zap.DebugLevel, "transaction"); ce != nil {
					ce.Write(
						zap.Any("body", t),
						zap.Any("sql", t.Sql()),
					)
				}
				return t, err

			case *pglogrepl.UpdateMessage:
				relationInfo, ok = c.relationMessages[logicalMsg.RelationID]
				if !ok {
					log.Fatalf("unknown relation ID %d", logicalMsg.RelationID)
				}

				log.Debugf("RELTYPE   %v", relationInfo)

				newValues := ColValsFromLogMsg(logicalMsg.NewTuple.Columns, relationInfo)
				//				log.Printf("DEBUG UPDATE %s.%s: %v", rel.Namespace, rel.RelationName, new_values)

				originalValues := ColValsFromLogMsg(logicalMsg.OldTuple.Columns, relationInfo)
				whereVals := WhereFromLogMsg(c.relationMessages[logicalMsg.RelationID].Columns, originalValues)
				t = Transaction{
					LSN:  uint64(xld.WALStart),
					Type: "UPDATE",
					Tables: Tables{Table{
						Namespace: relationInfo.Namespace,
						TableName: relationInfo.RelationName,
					}},
					Values: newValues,
					Where:  whereVals,
				}
				if ce := quickLog.Check(zap.DebugLevel, "transaction"); ce != nil {
					ce.Write(
						zap.Any("body", t),
						zap.Any("sql", t.Sql()),
					)
				}
				return t, err
			case *pglogrepl.DeleteMessage:
				relationInfo, ok = c.relationMessages[logicalMsg.RelationID]
				if !ok {
					log.Fatalf("unknown relation ID %d", logicalMsg.RelationID)
				}

				oldValues := ColValsFromLogMsg(logicalMsg.OldTuple.Columns, relationInfo)
				whereVals := WhereFromLogMsg(c.relationMessages[logicalMsg.RelationID].Columns, oldValues)
				t = Transaction{
					LSN:  uint64(xld.WALStart),
					Type: "DELETE",
					Tables: Tables{Table{
						Namespace: relationInfo.Namespace,
						TableName: relationInfo.RelationName,
					}},
					Where: whereVals,
				}
				if ce := quickLog.Check(zap.DebugLevel, "transaction"); ce != nil {
					ce.Write(
						zap.Any("body", t),
						zap.Any("sql", t.Sql()),
					)
				}
				return t, err

			case *pglogrepl.TruncateMessage:
				log.Debug(logicalMsg)
				var tables Tables
				var table Table
				for _, oid := range logicalMsg.RelationIDs {
					if table, err = c.GetTableFromOID(oid); err != nil {
						log.Fatalf("could not retrieve relation OID %d", logicalMsg.RelationNum)
					}
					tables = append(tables, table)
				}
				t = Transaction{
					LSN:    uint64(xld.WALStart),
					Type:   "TRUNCATE",
					Tables: tables,
				}
				if ce := quickLog.Check(zap.DebugLevel, "transaction"); ce != nil {
					ce.Write(
						zap.Any("body", t),
						zap.Any("sql", t.Sql()),
					)
				}
				return t, err

			case *pglogrepl.TypeMessage:
			case *pglogrepl.OriginMessage:
			default:
				log.Infof("Unknown message type in pgoutput stream: %T", logicalMsg)
			}

			c.XLogPos = xld.WALStart + pglogrepl.LSN(len(xld.WALData))
		}
	}
}
