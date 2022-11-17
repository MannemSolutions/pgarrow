package pg

/*
   Software is based on the pglogrepl_demo that is provided in the
	 git repository from Jack Christensen (https://github.com/jackc/pglogrepl).
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgtype"
)

type RelationColumnType struct {
	RelationID      uint32
	RelationColumns []int
}

// ColumnKeyValues is an array of columns and values that we will use to create  the keylist.
// The values are stored as string because a JSON is created anyway.
type ColumnKeyValue struct {
	ColumnKey   string
	ColumnValue string
}

// The JSON_Transaction struct is used as a format for storing
type JSON_Transaction struct {
	TransactionType   string
	RelationNamespace string
	RelationName      string
	ColumnList        map[string]interface{}
	FilterConditions  []ColumnKeyValue
}

func main() {
	const outputPlugin = "pgoutput"
	const CONN = "postgres://postgres:Secret!@vlhvmpgtst/x-park_db?replication=database"
	var relationColumns []RelationColumnType

	conn, err := pgconn.Connect(context.Background(), CONN)
	if err != nil {
		log.Fatalln("failed to connect to PostgreSQL server:", err)
	}
	defer conn.Close(context.Background())

	//	result := conn.Exec(context.Background(), "DROP PUBLICATION IF EXISTS pglogrepl_demo;")
	//	_, err = result.ReadAll()
	//	if err != nil {
	//		log.Fatalln("drop publication if exists error", err)
	//	}

	//	result = conn.Exec(context.Background(), "CREATE PUBLICATION pglogrepl_demo FOR ALL TABLES;")
	//	_, err = result.ReadAll()
	//	if err != nil {
	//		log.Fatalln("create publication error", err)
	//	}
	//	log.Println("create publication pglogrepl_demo")

	var pluginArguments []string
	if outputPlugin == "pgoutput" {
		pluginArguments = []string{"proto_version '1'", "publication_names 'pglogrepl_demo'"}
	} else if outputPlugin == "wal2json" {
		pluginArguments = []string{"\"pretty-print\" 'true'"}
	}

	// Currently the system is collecting the information about the systeem and starting replication
	// from that point. But probably we may need to store the XLogPos at some point to be able to
	// restart replication from that point onwards.

	sysident, err := pglogrepl.IdentifySystem(context.Background(), conn)
	if err != nil {
		log.Fatalln("IdentifySystem failed:", err)
	}
	log.Println("SystemID:", sysident.SystemID, "Timeline:", sysident.Timeline, "XLogPos:", sysident.XLogPos, "DBName:", sysident.DBName)

	slotName := "log_repl_slot"

	//	_, err = pglogrepl.CreateReplicationSlot(context.Background(), conn, slotName, outputPlugin, pglogrepl.CreateReplicationSlotOptions{Temporary: true})
	//	if err != nil {
	//		log.Fatalln("CreateReplicationSlot failed:", err)
	//	}
	// log.Println("Created temporary replication slot:", slotName)
	err = pglogrepl.StartReplication(context.Background(), conn, slotName, sysident.XLogPos, pglogrepl.StartReplicationOptions{PluginArgs: pluginArguments})
	if err != nil {
		log.Fatalln("StartReplication failed:", err)
	}
	log.Println("Logical replication started on slot", slotName)

	clientXLogPos := sysident.XLogPos
	standbyMessageTimeout := time.Second * 10
	nextStandbyMessageDeadline := time.Now().Add(standbyMessageTimeout)
	relations := map[uint32]*pglogrepl.RelationMessage{}
	connInfo := pgtype.NewConnInfo()

	for {
		if time.Now().After(nextStandbyMessageDeadline) {
			err = pglogrepl.SendStandbyStatusUpdate(context.Background(), conn, pglogrepl.StandbyStatusUpdate{WALWritePosition: clientXLogPos})
			if err != nil {
				log.Fatalln("SendStandbyStatusUpdate failed:", err)
			}
			// log.Println("Sent Standby status message")
			nextStandbyMessageDeadline = time.Now().Add(standbyMessageTimeout)
		}

		ctx, cancel := context.WithDeadline(context.Background(), nextStandbyMessageDeadline)
		rawMsg, err := conn.ReceiveMessage(ctx)
		cancel()
		if err != nil {
			if pgconn.Timeout(err) {
				continue
			}
			log.Fatalln("ReceiveMessage failed:", err)
		}

		if errMsg, ok := rawMsg.(*pgproto3.ErrorResponse); ok {
			fmt.Println("received Postgres WAL error: ", errMsg)
			return
		}

		msg, ok := rawMsg.(*pgproto3.CopyData)
		if !ok {
			log.Printf("Received unexpected message: %T\n", rawMsg)
			continue
		}

		switch msg.Data[0] {
		case pglogrepl.PrimaryKeepaliveMessageByteID:
			pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
			if err != nil {
				log.Fatalln("ParsePrimaryKeepaliveMessage failed:", err)
			}
			// log.Println("Primary Keepalive Message =>", "ServerWALEnd:", pkm.ServerWALEnd, "ServerTime:", pkm.ServerTime, "ReplyRequested:", pkm.ReplyRequested)

			if pkm.ReplyRequested {
				nextStandbyMessageDeadline = time.Time{}
			}

		case pglogrepl.XLogDataByteID:
			xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
			if err != nil {
				log.Fatalln("ParseXLogData failed:", err)
			}
			//log.Println("DEBUG XLogData =>", "WALStart", xld.WALStart, "ServerWALEnd", xld.ServerWALEnd, "ServerTime:", xld.ServerTime, "WALData", string(xld.WALData))
			logicalMsg, err := pglogrepl.Parse(xld.WALData)
			if err != nil {
				log.Fatalf("Parse logical replication message: %s", err)
			}
			log.Printf("Receive a logical replication message: %s", logicalMsg.Type())

			switch logicalMsg := logicalMsg.(type) {
			case *pglogrepl.RelationMessage:
				fmt.Println("RELATION MESSAGE")

				relations[logicalMsg.RelationID] = logicalMsg
				//				fmt.Printf("Getting information for relation %d (%s)\n", logicalMsg.RelationID, logicalMsg.RelationName)

				var cols []int

				for idx, col := range logicalMsg.Columns {
					if col.Flags == 1 {
						//					fmt.Printf("RELATION COLUMNS %d with flag=%d   %s\n", idx, col.Flags, col.Name)
						cols = append(cols, idx)
					}
				}
				//				fmt.Printf("My Array of relation columns: %v\n", cols)
				var thisRelation = RelationColumnType{logicalMsg.RelationID, cols}

				// We may need to reconsider already added relations. They should be replaced...
				relationColumns = append(relationColumns, thisRelation)

				//				fmt.Printf("Relation Message: %v\n", logicalMsg.Columns)
				//				fmt.Printf("All stored relations: %v\n", relationColumns)
			case *pglogrepl.BeginMessage:
				// Indicates the beginning of a group of changes in a transaction. This is only sent for committed transactions. You won't get any events from rolled back transactions.
				fmt.Println("BEGIN MESSAGE")

			case *pglogrepl.CommitMessage:
				fmt.Println("COMMIT MESSAGE")

			case *pglogrepl.InsertMessage:
				fmt.Println("INSERT MESSAGE")
				rel, ok := relations[logicalMsg.RelationID]
				if !ok {
					log.Fatalf("unknown relation ID %d", logicalMsg.RelationID)
				}

				relationJSON, _ := json.Marshal([]string{rel.Namespace, rel.RelationName})
				fmt.Printf("Relation JSON part: %s\n", relationJSON)

				values := map[string]interface{}{}
				for idx, col := range logicalMsg.Tuple.Columns {
					colName := rel.Columns[idx].Name
					switch col.DataType {
					case 'n': // null
						values[colName] = nil
					case 'u': // unchanged toast
						// This TOAST value was not changed. TOAST values are not stored in the tuple, and logical replication doesn't want to spend a disk read to fetch its value for you.
					case 't': //text
						val, err := decodeTextColumnData(connInfo, col.Data, rel.Columns[idx].DataType)
						if err != nil {
							log.Fatalln("error decoding column data:", err)
						}
						values[colName] = val
					}
				}
				log.Printf("INSERT INTO %s.%s: %v", rel.Namespace, rel.RelationName, values)
				jsonRecord, _ := json.Marshal(values)
				fmt.Printf("JSON part = %s\n", jsonRecord)

				var json_transaction JSON_Transaction

				json_transaction.TransactionType = "INSERT"
				json_transaction.RelationNamespace = rel.Namespace
				json_transaction.RelationName = rel.RelationName
				json_transaction.ColumnList = values
				json_transaction.FilterConditions = []ColumnKeyValue{}

				MyOutput, _ := json.Marshal(json_transaction)
				fmt.Printf("Full JSON = %s\n", MyOutput)

			case *pglogrepl.UpdateMessage:
				fmt.Println("UPDATE MESSAGE")

				var updateKeyList []ColumnKeyValue

				rel, ok := relations[logicalMsg.RelationID]
				if !ok {
					log.Fatalf("unknown relation ID %d", logicalMsg.RelationID)
				}

				relationJSON, _ := json.Marshal([]string{rel.Namespace, rel.RelationName})
				fmt.Printf("Relation JSON part: %s\n", relationJSON)

				// fmt.Printf("RELTYPE   %v\n", rel)

				new_values := map[string]interface{}{}
				for idx, col := range logicalMsg.NewTuple.Columns {
					colName := rel.Columns[idx].Name
					switch col.DataType {
					case 'n': // null
						new_values[colName] = nil
					case 'u': // unchanged toast
						// This TOAST value was not changed. TOAST values are not stored in the tuple, and logical replication doesn't want to spend a disk read to fetch its value for you.
					case 't': //text
						val, err := decodeTextColumnData(connInfo, col.Data, rel.Columns[idx].DataType)
						if err != nil {
							log.Fatalln("error decoding column data:", err)
						}
						new_values[colName] = val
					}
				}

				//				log.Printf("DEBUG UPDATE %s.%s: %v", rel.Namespace, rel.RelationName, new_values)
				jsonRowRecord, _ := json.Marshal(new_values)
				fmt.Printf("JSON part = %s\n", jsonRowRecord)

				for i := range relationColumns {
					if relationColumns[i].RelationID == logicalMsg.RelationID {
						//						fmt.Println("Relation found in stored relations (metadata about columns)")
						//						fmt.Printf("Columnlist is: %v\n", relationColumns[i].RelationColumns)
						//						fmt.Printf("New values: %v\n", new_values)
						for r := range relationColumns[i].RelationColumns {
							fmt.Printf("Update row with columns column %s = %v\n", rel.Columns[r].Name, new_values[rel.Columns[r].Name])
							var myColumnKeyValue ColumnKeyValue
							myColumnKeyValue.ColumnKey = rel.Columns[r].Name
							myColumnKeyValue.ColumnValue = fmt.Sprintf("%v", new_values[rel.Columns[r].Name])

							updateKeyList = append(updateKeyList, myColumnKeyValue)
						}
						var json_transaction JSON_Transaction

						json_transaction.TransactionType = "UPDATE"
						json_transaction.RelationNamespace = rel.Namespace
						json_transaction.RelationName = rel.RelationName
						json_transaction.ColumnList = new_values
						json_transaction.FilterConditions = updateKeyList

						MyOutput, _ := json.Marshal(json_transaction)
						fmt.Printf("Full JSON = %s\n", MyOutput)
						break

					}
				}

				jsonRecord, _ := json.Marshal(updateKeyList)
				fmt.Printf("JSON part = %s\n", jsonRecord)

			case *pglogrepl.DeleteMessage:
				fmt.Println("DELETE MESSAGE")

				rel, ok := relations[logicalMsg.RelationID]
				if !ok {
					log.Fatalf("unknown relation ID %d", logicalMsg.RelationID)
				}

				relationJSON, _ := json.Marshal([]string{rel.Namespace, rel.RelationName})
				fmt.Printf("Relation JSON part: %s\n", relationJSON)

				old_values := map[string]interface{}{}

				//
				// Dumping information about the OldTuple data
				//
				//				fmt.Printf("Old Tuple Type: %v\n", logicalMsg.OldTupleType)
				for idx, col := range logicalMsg.OldTuple.Columns {
					colName := rel.Columns[idx].Name
					switch col.DataType {
					case 'n': // null
						old_values[colName] = nil
					case 'u': // unchanged toast
						// This TOAST value was not changed. TOAST values are not stored in the tuple, and logical replication doesn't want to spend a disk read to fetch its value for you.
					case 't': //text
						val, err := decodeTextColumnData(connInfo, col.Data, rel.Columns[idx].DataType)
						if err != nil {
							log.Fatalln("error decoding column data:", err)
						}
						old_values[colName] = val
					}
				}

				// Searching for the relation and it's primary key
				//				fmt.Println("Relation colums:")

				var deleteKeyList []ColumnKeyValue

				// Linear search through all stored relations to find our relation
				for i := range relationColumns {
					if relationColumns[i].RelationID == logicalMsg.RelationID {
						//						fmt.Println("Relation found in stored relations (metadata about columns)")
						//						fmt.Printf("Columnlist is: %v\n", relationColumns[i].RelationColumns)
						//						fmt.Printf("%v\n", old_values)

						for r := range relationColumns[i].RelationColumns {
							fmt.Printf("Old column %s with value %v\n", rel.Columns[r].Name, old_values[rel.Columns[r].Name])
							var myColumnKeyValue ColumnKeyValue
							myColumnKeyValue.ColumnKey = rel.Columns[r].Name
							myColumnKeyValue.ColumnValue = fmt.Sprintf("%v", old_values[rel.Columns[r].Name])

							deleteKeyList = append(deleteKeyList, myColumnKeyValue)
						}
						var json_transaction JSON_Transaction

						json_transaction.TransactionType = "DELETE"
						json_transaction.RelationNamespace = rel.Namespace
						json_transaction.RelationName = rel.RelationName
						json_transaction.ColumnList = map[string]interface{}{}
						json_transaction.FilterConditions = deleteKeyList

						//						jsonRecord, _ := json.Marshal(deleteKeyList)
						MyOutput, _ := json.Marshal(json_transaction)

						//						fmt.Printf("JSON part = %s\n", jsonRecord)
						fmt.Printf("Full JSON = %s\n", MyOutput)
						break // Leave the loop because we found the relation
					}
				}

			case *pglogrepl.TruncateMessage:
				fmt.Println("TRUNCATE MESSSAGE")

			case *pglogrepl.TypeMessage:
				fmt.Println("TYPE MESSSAGE")
			case *pglogrepl.OriginMessage:
				fmt.Println("ORIGINAL MESSSAGE")
			default:
				log.Printf("Unknown message type in pgoutput stream: %T", logicalMsg)
			}

			clientXLogPos = xld.WALStart + pglogrepl.LSN(len(xld.WALData))
		}
	}
}

func decodeTextColumnData(ci *pgtype.ConnInfo, data []byte, dataType uint32) (interface{}, error) {
	var decoder pgtype.TextDecoder
	if dt, ok := ci.DataTypeForOID(dataType); ok {
		decoder, ok = dt.Value.(pgtype.TextDecoder)
		if !ok {
			decoder = &pgtype.GenericText{}
		}
	} else {
		decoder = &pgtype.GenericText{}
	}
	if err := decoder.DecodeText(ci, data); err != nil {
		return nil, err
	}
	return decoder.(pgtype.Value).Get(), nil
}
