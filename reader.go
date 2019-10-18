package main

import (
	"fmt"

	"github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/replication"
)

type BinlogReader struct {
	Config replication.BinlogSyncerConfig
	GTID   mysql.GTIDSet
	Syncer *replication.BinlogSyncer
}

func NewBinlogReader() (binlogReader *BinlogReader, err error) {
	binlogReader = &BinlogReader{
		Config: replication.BinlogSyncerConfig{
			Flavor:   "mysql",
			Host:     Config.MySQL.Host,
			Port:     Config.MySQL.Port,
			User:     Config.MySQL.User,
			Password: Config.MySQL.Password,
			ServerID: Config.MySQL.ServerID,
		},
	}
	gtid, err := Config.getMasterGTIDSet()
	if err != nil {
		return nil, fmt.Errorf("unable to get MySQL executed GTIDs: %v", err)
	}
	binlogReader.GTID = gtid
	binlogReader.Syncer = replication.NewBinlogSyncer(binlogReader.Config)
	return binlogReader, nil
}

func (r *BinlogReader) StartSync() (binlogStreamer *replication.BinlogStreamer, err error) {
	return r.Syncer.StartSyncGTID(r.GTID)
}

func (r *BinlogReader) Close() {
	r.Syncer.Close()
}
