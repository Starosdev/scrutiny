package database

const (
    // Viper config keys
    cfgInfluxDBOrg             = "web.influxdb.org"
    cfgInfluxDBBucket          = "web.influxdb.bucket"
    cfgInfluxDBRetentionPolicy = "web.influxdb.retention_policy"
    cfgDatabaseLocation        = "web.database.location"

    // GORM query conditions
    queryDeviceID = "device_id = ?"
    queryGUID     = "guid = ?"

    // Error format strings
    errDeviceNotFound  = "could not get device from DB: %v"
    errZFSPoolNotFound = "could not get ZFS pool from DB: %v"
)
