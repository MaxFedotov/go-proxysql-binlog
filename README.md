# go-proxysql-binlog
Go version of https://github.com/sysown/proxysql_mysqlbinlog/

## Features
- Improved logging and error handling
- Prometheus metrics endpoint
- Pre-build deb and rpm packages

## Download
- Get the latest binary\rpm\deb package from the [releases](https://github.com/MaxFedotov/go-proxysql-binlog/releases)
- Or get and complile from sources
```shell
go get github.com/MaxFedotov/go-proxysql-binlog
cd {module directory}
./build.sh -t linux -d
```

## Install notes
Install proxysql-binlog on MySQL host. Configure MySQL (see [MySQL configuration](#mysql-configuration)) and 
ProxySQL (see [ProxySQL configuration](#proxysql-configuration))

## Usage
```
Usage of proxysq_binlog:
  -config string
    	Path to config file (default "/etc/proxysql-binlog.cnf")
  -debug
    	Debug mode
  -version
    	Print version
```

## Configuration
```
[general]
listen_address = "0.0.0.0:3310"                             # Address to listen for connections from ProxySQL
log_file = "/var/log/proxysql-binlog/proxysql-binlog.log"   # Path to proxysql-binlog log file
log_level = "info"                                          # Log level

[mysql]
host = "localhost"                                          # MySQL host
port = 3306                                                 # MySQL port
user = "slave"                                              # MySQL user
password = "slavepassword"                                  # MySQL password

[metrics]
listen_address = "0.0.0.0:9056"                             # Address to listen on for metrics web interface
endpoint = "/metrics"                                       # Path under which to expose metrics
```

## MySQL configuration
Create user for proxysql-binlog and grant `REPLICATION SLAVE` permissions (if you already have slave user - you can use it and skip this step):
```sql
CREATE USER 'slave'@'localhost' IDENTIFIED BY 'slavepassword';
GRANT REPLICATION SLAVE ON *.* TO 'slave'@'localhost';
```
Add following lines to my.cnf:
```
log-bin                  = mysql-bin
enforce_gtid_consistency = ON
binlog_row_image         = FULL
gtid_mode                = ON
session_track_gtids      = OWN_GTID
```

## ProxySQL configuration
Assuming that you have following ProxySQL configuration (2 servers in 2 different hostgroups, one master and one slave)
```sql
      SELECT hostgroup_id, hostname, gtid_port FROM mysql_servers;
      +--------------+-----------------------------+-----------+
      | hostgroup_id | hostname                    | gtid_port |
      +--------------+-----------------------------+-----------+
      | 1            | myserver-master.local       | 3333      |
      | 2            | myserver-slave.local        | 3333      |
      +--------------+-----------------------------+-----------+
```
Update ProxySQL mysql_servers configuration and set gtid_port for servers:
```sql
      UPDATE mysql_servers SET gtid_port=3310;
      LOAD MYSQL SERVERS TO RUNTIME;
      SAVE MYSQL SERVERS TO DISK; 
```
You can check that everything is working using following query:
```sql
      SELECT * FROM stats_mysql_gtid_executed;
      +-----------------------------+------+-------------------------------------------+--------+
      | hostname                    | port | gtid_executed                             | events |
      +-----------------------------+------+-------------------------------------------+--------+
      | myserver-master.local       | 3306 | eb3fe955-a267-11e9-b32c-fa163e7593dc:1-68 | 1      |
      | myserver-slave.local        | 3306 | eb3fe955-a267-11e9-b32c-fa163e7593dc:1-68 | 1      |
      +-----------------------------+------+-------------------------------------------+--------+
```
In order for start using GTID casual reads you need to create following rules in Proxysql:
```sql
      INSERT INTO mysql_query_rules(rule_id,active,match_digest,destination_hostgroup,gtid_from_hostgroup) VALUES(1,1,'^SELECT.*FOR UPDATE',1,NULL);
      INSERT INTO mysql_query_rules(rule_id,active,match_digest,destination_hostgroup,gtid_from_hostgroup) VALUES(2,1,'^SELECT',2,1);
      LOAD MYSQL QUERY RULES TO RUNTIME;
      SAVE MYSQL QUERY RULES TO DISK;
```
You can verify request routing using following query:
```sql
      SELECT hostgroup, srv_host, Queries, Queries_GTID_sync FROM stats_mysql_connection_pool;
```

