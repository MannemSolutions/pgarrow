package pg

/*
   Software is based on the pglogrepl_demo that is provided in the
	 git repository from Jack Christensen (https://github.com/jackc/pglogrepl).
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgproto3"
)

func (c Conn) StartRepl() (err error) {
	if err = c.Connect(); err != nil {
		return err
	}

	_, err = pglogrepl.CreateReplicationSlot(context.Background(), c.conn, c.config.Slot, "pgoutput",
		pglogrepl.CreateReplicationSlotOptions{Temporary: true})
	if err != nil {
		log.Fatalln("CreateReplicationSlot failed:", err)
	}
	log.Info("Created temporary replication slot:", c.config.Slot)
	err = pglogrepl.StartReplication(
		context.Background(),
		c.conn,
		c.config.Slot,
		c.sysIdent.XLogPos,
		pglogrepl.StartReplicationOptions{
			PluginArgs: []string{"proto_version '1'", "publication_names 'pgarrow'"}})
	if err != nil {
		log.Fatalln("StartReplication failed:", err)
	}
	log.Info("Logical replication started on slot", c.config.Slot)
	return nil
}

func (c *Conn) NextTransaction() (t Transaction, err error) {
	standbyMessageTimeout := time.Millisecond * time.Duration(c.config.standbyMessageTimeout)
	nextStandbyMessageDeadline := time.Now().Add(standbyMessageTimeout)

	for {
		if time.Now().After(nextStandbyMessageDeadline) {
			err = pglogrepl.SendStandbyStatusUpdate(context.Background(), c.conn, pglogrepl.StandbyStatusUpdate{WALWritePosition: c.sysIdent.XLogPos})
			if err != nil {
				log.Fatalln("SendStandbyStatusUpdate failed:", err)
			}
			log.Debug("Sent Standby status message")
			nextStandbyMessageDeadline = time.Now().Add(standbyMessageTimeout)
		}

		ctx, cancel := context.WithDeadline(context.Background(), nextStandbyMessageDeadline)
		rawMsg, err := c.conn.ReceiveMessage(ctx)
		cancel()
		if err != nil {
			if pgconn.Timeout(err) {
				continue
			}
			log.Info("ReceiveMessage failed:", err)
			return t, err
		}

		if errMsg, ok := rawMsg.(*pgproto3.ErrorResponse); ok {
			log.Debug("received Postgres WAL error: ", errMsg)
			return t, err
		}

		msg, ok := rawMsg.(*pgproto3.CopyData)
		if !ok {
			log.Infof("Received unexpected message: %T\n", rawMsg)
			continue
		}

		switch msg.Data[0] {
		case pglogrepl.PrimaryKeepaliveMessageByteID:
			pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
			if err != nil {
				log.Fatalln("ParsePrimaryKeepaliveMessage failed:", err)
			}
			log.Debug("Primary Keepalive Message =>", "ServerWALEnd:",
				pkm.ServerWALEnd, "ServerTime:",
				pkm.ServerTime, "ReplyRequested:",
				pkm.ReplyRequested)

			if pkm.ReplyRequested {
				nextStandbyMessageDeadline = time.Time{}
			}

		case pglogrepl.XLogDataByteID:
			xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
			if err != nil {
				log.Fatalln("ParseXLogData failed:", err)
			}
			log.Debug("XLogData =>", "WALStart",
				xld.WALStart,
				"ServerWALEnd",
				xld.ServerWALEnd,
				"ServerTime:", xld.ServerTime,
				"WALData", string(xld.WALData))
			logicalMsg, err := pglogrepl.Parse(xld.WALData)
			if err != nil {
				log.Fatalf("Parse logical replication message: %s", err)
			}
			log.Infof("Receive a logical replication message: %s", logicalMsg.Type())

			switch logicalMsg := logicalMsg.(type) {
			case *pglogrepl.RelationMessage:
				log.Debug("RELATION MESSAGE")

				c.relationMessages[logicalMsg.RelationID] = logicalMsg
			case *pglogrepl.BeginMessage:
				// Indicates the beginning of a group of changes in a transaction. This is only sent for committed transactions. You won't get any events from rolled back transactions.
				log.Debug("BEGIN MESSAGE")

			case *pglogrepl.CommitMessage:
				log.Debug("COMMIT MESSAGE")

			case *pglogrepl.InsertMessage:
				log.Debug("INSERT MESSAGE")
				relationInfo, ok := c.relationMessages[logicalMsg.RelationID]
				if !ok {
					log.Fatalf("unknown relation ID %d", logicalMsg.RelationID)
				}

				relationJSON, _ := json.Marshal([]string{relationInfo.Namespace, relationInfo.RelationName})
				log.Debug("Relation JSON part: %s\n", relationJSON)

				newValues := ColValsFromLogMsg(logicalMsg.Tuple.Columns, relationInfo)
				log.Debug("INSERT INTO %s.%s: %v", relationInfo.Namespace, relationInfo.RelationName, relationInfo)
				jsonRecord, _ := json.Marshal(relationInfo)
				log.Debug("JSON part = %s\n", jsonRecord)

				t = Transaction{
					Type:      "INSERT",
					Namespace: relationInfo.Namespace,
					RelName:   relationInfo.RelationName,
					Vals:      newValues,
				}
				return t, err

			case *pglogrepl.UpdateMessage:
				log.Debug("UPDATE MESSAGE")

				relationInfo, ok := c.relationMessages[logicalMsg.RelationID]
				if !ok {
					log.Fatalf("unknown relation ID %d", logicalMsg.RelationID)
				}

				relationJSON, _ := json.Marshal([]string{relationInfo.Namespace, relationInfo.RelationName})
				log.Debug("Relation JSON part: %s\n", relationJSON)

				log.Debug("RELTYPE   %v\n", relationInfo)

				newValues := ColValsFromLogMsg(logicalMsg.NewTuple.Columns, relationInfo)
				//				log.Printf("DEBUG UPDATE %s.%s: %v", rel.Namespace, rel.RelationName, new_values)
				jsonRowRecord, _ := json.Marshal(newValues)
				log.Debug("JSON part = %s\n", jsonRowRecord)

				originalValues := ColValsFromLogMsg(logicalMsg.OldTuple.Columns, relationInfo)
				whereVals := WhereFromLogMsg(c.relationMessages[logicalMsg.RelationID].Columns, originalValues)
				t = Transaction{
					Type:      "UPDATE",
					Namespace: relationInfo.Namespace,
					RelName:   relationInfo.RelationName,
					Vals:      newValues,
					Where:     whereVals,
				}
				return t, err
			case *pglogrepl.DeleteMessage:
				log.Debug("DELETE MESSAGE")

				relationInfo, ok := c.relationMessages[logicalMsg.RelationID]
				if !ok {
					log.Fatalf("unknown relation ID %d", logicalMsg.RelationID)
				}

				relationJSON, _ := json.Marshal([]string{relationInfo.Namespace, relationInfo.RelationName})
				log.Debug("Relation JSON part: %s\n", relationJSON)

				oldValues := ColValsFromLogMsg(logicalMsg.OldTuple.Columns, relationInfo)
				whereVals := WhereFromLogMsg(c.relationMessages[logicalMsg.RelationID].Columns, oldValues)
				t = Transaction{
					Type:      "DELETE",
					Namespace: relationInfo.Namespace,
					RelName:   relationInfo.RelationName,
					Where:     whereVals,
				}
				return t, err

			case *pglogrepl.TruncateMessage:
				log.Debug("TRUNCATE MESSSAGE")
				rel, ok := c.relationMessages[logicalMsg.RelationNum]
				if !ok {
					log.Fatalf("unknown relation ID %d", logicalMsg.RelationNum)
				}
				t = Transaction{
					Type:      "TRUNCATE",
					Namespace: rel.Namespace,
					RelName:   rel.RelationName,
				}
				return t, nil

			case *pglogrepl.TypeMessage:
				fmt.Println("TYPE MESSSAGE")
			case *pglogrepl.OriginMessage:
				fmt.Println("ORIGINAL MESSSAGE")
			default:
				log.Infof("Unknown message type in pgoutput stream: %T", logicalMsg)
			}

			c.sysIdent.XLogPos = xld.WALStart + pglogrepl.LSN(len(xld.WALData))
		}
	}
}
