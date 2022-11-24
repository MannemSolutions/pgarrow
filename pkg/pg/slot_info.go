package pg

import "github.com/jackc/pglogrepl"

type slotInfos map[string]slotInfo
type slotInfo struct {
	name       string
	active     bool
	restartLsn pglogrepl.LSN
}
