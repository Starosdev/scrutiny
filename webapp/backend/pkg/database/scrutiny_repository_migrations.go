package database

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20201107210306"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20220503120000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20220509170100"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20220716214900"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20250221084400"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20251108044508"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260108000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260122000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260129000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260131000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260202000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260225000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260226000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260301000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260315000000"
	_ "github.com/analogj/scrutiny/webapp/backend/pkg/database/migrations/m20260401000000"
	"github.com/analogj/scrutiny/webapp/backend/pkg/deviceid"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	_ "github.com/glebarez/sqlite"
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/http"
	"gorm.io/gorm"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// SQLite migrations
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//database.AutoMigrate(&models.Device{})

func (sr *scrutinyRepository) Migrate(ctx context.Context) error {

	sr.logger.Infoln("Database migration starting. Please wait, this process may take a long time....")

	gormMigrateOptions := gormigrate.DefaultOptions
	gormMigrateOptions.UseTransaction = true

	m := gormigrate.New(sr.gormClient, gormMigrateOptions, []*gormigrate.Migration{
		{
			ID: "20201107210306", // v0.3.13 (pre-influxdb schema). 9fac3c6308dc6cb6cd5bbc43a68cd93e8fb20b87
			Migrate: func(tx *gorm.DB) error {
				// it's a good practice to copy the struct inside the function,

				return tx.AutoMigrate(
					&m20201107210306.Device{},
					&m20201107210306.Smart{},
					&m20201107210306.SmartAtaAttribute{},
					&m20201107210306.SmartNvmeAttribute{},
					&m20201107210306.SmartNvmeAttribute{},
				)
			},
		},
		{
			ID: "20220503113100", // backwards compatible - influxdb schema
			Migrate: func(tx *gorm.DB) error {
				// delete unnecessary table.
				err := tx.Migrator().DropTable("self_tests")
				if err != nil {
					return err
				}

				//add columns to the Device schema, so we can start adding data to the database & influxdb
				err = tx.Migrator().AddColumn(&models.Device{}, "Label") //Label  string `json:"label"`
				if err != nil {
					return err
				}
				err = tx.Migrator().AddColumn(&models.Device{}, "DeviceStatus") //DeviceStatus pkg.DeviceStatus `json:"device_status"`
				if err != nil {
					return err
				}

				//TODO: migrate the data from GORM to influxdb.
				//get a list of all devices:
				//	get a list of all smart scans in the last 2 weeks:
				//		get a list of associated smart attribute data:
				//			translate to a measurements.Smart{} object
				//			call CUSTOM INFLUXDB SAVE FUNCTION (taking bucket as parameter)
				//	get a list of all smart scans in the last 9 weeks:
				//		do same as above (select 1 scan per week)
				//	get a list of all smart scans in the last 25 months:
				//		do same as above (select 1 scan per month)
				//	get a list of all smart scans:
				//		do same as above (select 1 scan per year)

				preDevices := []m20201107210306.Device{} //pre-migration device information
				if err = tx.Preload("SmartResults", func(db *gorm.DB) *gorm.DB {
					return db.Order("smarts.created_at ASC") //OLD: .Limit(devicesCount)
				}).Find(&preDevices).Error; err != nil {
					sr.logger.Errorln("Could not get device summary from DB", err)
					return err
				}

				//calculate bucket oldest dates
				today := time.Now()
				dailyBucketMax := today.Add(-DEFAULT_RETENTION_PERIOD_15_DAYS_IN_SECONDS * time.Second)     //15 days
				weeklyBucketMax := today.Add(-DEFAULT_RETENTION_PERIOD_9_WEEKS_IN_SECONDS * time.Second)    //9 weeks
				monthlyBucketMax := today.Add(-DEFAULT_RETENTION_PERIOD_25_MONTHS_IN_SECONDS * time.Second) //25 months

				for _, preDevice := range preDevices {
					sr.logger.Debugf("====================================")
					sr.logger.Infof("begin processing device: %s", preDevice.WWN)

					//weekly, monthly, yearly lookup storage, so we don't add more data to the buckets than necessary.
					weeklyLookup := map[string]bool{}
					monthlyLookup := map[string]bool{}
					yearlyLookup := map[string]bool{}
					for _, preSmartResult := range preDevice.SmartResults { //pre-migration smart results

						//we're looping in ASC mode, so from oldest entry to most current.

						err, postSmartResults := m20201107210306_FromPreInfluxDBSmartResultsCreatePostInfluxDBSmartResults(tx, preDevice, preSmartResult)
						if err != nil {
							return err
						}
						smartTags, smartFields := postSmartResults.Flatten()

						err, postSmartTemp := m20201107210306_FromPreInfluxDBTempCreatePostInfluxDBTemp(preDevice, preSmartResult)
						if err != nil {
							return err
						}
						tempTags, tempFields := postSmartTemp.Flatten()
						tempTags["device_wwn"] = preDevice.WWN

						year, week := postSmartResults.Date.ISOWeek()
						month := postSmartResults.Date.Month()

						yearStr := strconv.Itoa(year)
						yearMonthStr := fmt.Sprintf("%d-%d", year, month)
						yearWeekStr := fmt.Sprintf("%d-%d", year, week)

						//write data to daily bucket if in the last 15 days
						if postSmartResults.Date.After(dailyBucketMax) {
							sr.logger.Debugf("device (%s) smart data added to bucket: daily", preDevice.WWN)
							// write point immediately
							err = sr.saveDatapoint(
								sr.influxClient.WriteAPIBlocking(sr.appConfig.GetString(cfgInfluxDBOrg), sr.appConfig.GetString(cfgInfluxDBBucket)),
								"smart",
								smartTags,
								smartFields,
								postSmartResults.Date, ctx)
							if sr.ignorePastRetentionPolicyError(err) != nil {
								return err
							}

							err = sr.saveDatapoint(
								sr.influxClient.WriteAPIBlocking(sr.appConfig.GetString(cfgInfluxDBOrg), sr.appConfig.GetString(cfgInfluxDBBucket)),
								"temp",
								tempTags,
								tempFields,
								postSmartResults.Date, ctx)
							if sr.ignorePastRetentionPolicyError(err) != nil {
								return err
							}
						}

						//write data to the weekly bucket if in the last 9 weeks, and week has not been processed yet
						if _, weekExists := weeklyLookup[yearWeekStr]; !weekExists && postSmartResults.Date.After(weeklyBucketMax) {
							sr.logger.Debugf("device (%s) smart data added to bucket: weekly", preDevice.WWN)

							//this week/year pair has not been processed
							weeklyLookup[yearWeekStr] = true
							// write point immediately
							err = sr.saveDatapoint(
								sr.influxClient.WriteAPIBlocking(sr.appConfig.GetString(cfgInfluxDBOrg), fmt.Sprintf("%s_weekly", sr.appConfig.GetString(cfgInfluxDBBucket))),
								"smart",
								smartTags,
								smartFields,
								postSmartResults.Date, ctx)

							if sr.ignorePastRetentionPolicyError(err) != nil {
								return err
							}

							err = sr.saveDatapoint(
								sr.influxClient.WriteAPIBlocking(sr.appConfig.GetString(cfgInfluxDBOrg), fmt.Sprintf("%s_weekly", sr.appConfig.GetString(cfgInfluxDBBucket))),
								"temp",
								tempTags,
								tempFields,
								postSmartResults.Date, ctx)
							if sr.ignorePastRetentionPolicyError(err) != nil {
								return err
							}
						}

						//write data to the monthly bucket if in the last 9 weeks, and week has not been processed yet
						if _, monthExists := monthlyLookup[yearMonthStr]; !monthExists && postSmartResults.Date.After(monthlyBucketMax) {
							sr.logger.Debugf("device (%s) smart data added to bucket: monthly", preDevice.WWN)

							//this month/year pair has not been processed
							monthlyLookup[yearMonthStr] = true
							// write point immediately
							err = sr.saveDatapoint(
								sr.influxClient.WriteAPIBlocking(sr.appConfig.GetString(cfgInfluxDBOrg), fmt.Sprintf("%s_monthly", sr.appConfig.GetString(cfgInfluxDBBucket))),
								"smart",
								smartTags,
								smartFields,
								postSmartResults.Date, ctx)
							if sr.ignorePastRetentionPolicyError(err) != nil {
								return err
							}

							err = sr.saveDatapoint(
								sr.influxClient.WriteAPIBlocking(sr.appConfig.GetString(cfgInfluxDBOrg), fmt.Sprintf("%s_monthly", sr.appConfig.GetString(cfgInfluxDBBucket))),
								"temp",
								tempTags,
								tempFields,
								postSmartResults.Date, ctx)
							if sr.ignorePastRetentionPolicyError(err) != nil {
								return err
							}
						}

						if _, yearExists := yearlyLookup[yearStr]; !yearExists && year != today.Year() {
							sr.logger.Debugf("device (%s) smart data added to bucket: yearly", preDevice.WWN)

							//this year has not been processed
							yearlyLookup[yearStr] = true
							// write point immediately
							err = sr.saveDatapoint(
								sr.influxClient.WriteAPIBlocking(sr.appConfig.GetString(cfgInfluxDBOrg), fmt.Sprintf("%s_yearly", sr.appConfig.GetString(cfgInfluxDBBucket))),
								"smart",
								smartTags,
								smartFields,
								postSmartResults.Date, ctx)
							if sr.ignorePastRetentionPolicyError(err) != nil {
								return err
							}

							err = sr.saveDatapoint(
								sr.influxClient.WriteAPIBlocking(sr.appConfig.GetString(cfgInfluxDBOrg), fmt.Sprintf("%s_yearly", sr.appConfig.GetString(cfgInfluxDBBucket))),
								"temp",
								tempTags,
								tempFields,
								postSmartResults.Date, ctx)
							if sr.ignorePastRetentionPolicyError(err) != nil {
								return err
							}
						}
					}
					sr.logger.Infof("finished processing device %s. weekly: %d, monthly: %d, yearly: %d", preDevice.WWN, len(weeklyLookup), len(monthlyLookup), len(yearlyLookup))

				}

				return nil
			},
		},
		{
			ID: "20220503120000", // cleanup - v0.4.0 - influxdb schema
			Migrate: func(tx *gorm.DB) error {
				// delete unnecessary tables.
				err := tx.Migrator().DropTable(
					&m20201107210306.Smart{},
					&m20201107210306.SmartAtaAttribute{},
					&m20201107210306.SmartNvmeAttribute{},
					&m20201107210306.SmartScsiAttribute{},
				)
				if err != nil {
					return err
				}

				//migrate the device database
				return tx.AutoMigrate(m20220503120000.Device{})
			},
		},
		{
			ID: "m20220509170100", // addl udev device data
			Migrate: func(tx *gorm.DB) error {

				//migrate the device database.
				// adding addl columns (device_label, device_uuid, device_serial_id)
				return tx.AutoMigrate(m20220509170100.Device{})
			},
		},
		{
			ID: "m20220709181300",
			Migrate: func(tx *gorm.DB) error {

				// delete devices with empty `wwn` field (they are impossible to delete manually), and are invalid.
				return tx.Where("wwn = ?", "").Delete(&models.Device{}).Error
			},
		},
		{
			ID: "m20220716214900", // add settings table.
			Migrate: func(tx *gorm.DB) error {

				// adding the settings table.
				err := tx.AutoMigrate(m20220716214900.Setting{})
				if err != nil {
					return err
				}
				//add defaults.

				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "theme",
						SettingKeyDescription: "Frontend theme ('light' | 'dark' | 'system')",
						SettingDataType:       "string",
						SettingValueString:    "system", // options: 'light' | 'dark' | 'system'
					},
					{
						SettingKeyName:        "layout",
						SettingKeyDescription: "Frontend layout ('material')",
						SettingDataType:       "string",
						SettingValueString:    "material",
					},
					{
						SettingKeyName:        "dashboard_display",
						SettingKeyDescription: "Frontend device display title ('name' | 'serial_id' | 'uuid' | 'label')",
						SettingDataType:       "string",
						SettingValueString:    "name",
					},
					{
						SettingKeyName:        "dashboard_sort",
						SettingKeyDescription: "Frontend device sort by ('status' | 'title' | 'age')",
						SettingDataType:       "string",
						SettingValueString:    "status",
					},
					{
						SettingKeyName:        "temperature_unit",
						SettingKeyDescription: "Frontend temperature unit ('celsius' | 'fahrenheit')",
						SettingDataType:       "string",
						SettingValueString:    "celsius",
					},
					{
						SettingKeyName:        "file_size_si_units",
						SettingKeyDescription: "File size in SI units (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      false,
					},
					{
						SettingKeyName:        "line_stroke",
						SettingKeyDescription: "Temperature chart line stroke ('smooth' | 'straight' | 'stepline')",
						SettingDataType:       "string",
						SettingValueString:    "smooth",
					},
					{
						SettingKeyName:        "metrics.notify_level",
						SettingKeyDescription: "Determines which device status will cause a notification (fail or warn)",
						SettingDataType:       "numeric",
						SettingValueNumeric:   int(pkg.MetricsNotifyLevelFail), // options: 'fail' or 'warn'
					},
					{
						SettingKeyName:        "metrics.status_filter_attributes",
						SettingKeyDescription: "Determines which attributes should impact device status",
						SettingDataType:       "numeric",
						SettingValueNumeric:   int(pkg.MetricsStatusFilterAttributesAll), // options: 'all' or  'critical'
					},
					{
						SettingKeyName:        "metrics.status_threshold",
						SettingKeyDescription: "Determines which threshold should impact device status",
						SettingDataType:       "numeric",
						SettingValueNumeric:   int(pkg.MetricsStatusThresholdBoth), // options: 'scrutiny', 'smart', 'both'
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
		{
			ID: "m20221115214900", // add line_stroke setting.
			Migrate: func(tx *gorm.DB) error {
				//add line_stroke setting default.
				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "line_stroke",
						SettingKeyDescription: "Temperature chart line stroke ('smooth' | 'straight' | 'stepline')",
						SettingDataType:       "string",
						SettingValueString:    "smooth",
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
		{
			ID: "m20231123123300", // add repeat_notifications setting.
			Migrate: func(tx *gorm.DB) error {
				//add repeat_notifications setting default.
				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "metrics.repeat_notifications",
						SettingKeyDescription: "Whether to repeat all notifications or just when values change (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      true,
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
		{
			ID: "m20240722082740", // add powered_on_hours_unit setting.
			Migrate: func(tx *gorm.DB) error {
				//add powered_on_hours_unit setting default.
				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "powered_on_hours_unit",
						SettingKeyDescription: "Presentation format for device powered on time ('humanize' | 'device_hours')",
						SettingDataType:       "string",
						SettingValueString:    "humanize",
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
		{
			ID: "m20250221084400", // add archived to device data
			Migrate: func(tx *gorm.DB) error {

				//migrate the device database.
				// adding column (archived)
				return tx.AutoMigrate(m20250221084400.Device{})
			},
		},
		{
			ID: "m20250609210800", // add retrieve_sct_history setting.
			Migrate: func(tx *gorm.DB) error {
				//add retrieve_sct_history setting default.
				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "collector.retrieve_sct_temperature_history",
						SettingKeyDescription: "Whether to retrieve SCT Temperature history (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      true,
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
		{
			ID: "m20251108044508", // add muted to device data
			Migrate: func(tx *gorm.DB) error {
				//migrate the device database.
				// adding column (muted)
				return tx.AutoMigrate(m20251108044508.Device{})
			},
		},
		{
			ID: "m20260108000000", // add ZFS pool and vdev tables
			Migrate: func(tx *gorm.DB) error {
				// Create ZFS pool and vdev tables
				return tx.AutoMigrate(
					&m20260108000000.ZFSPool{},
					&m20260108000000.ZFSVdev{},
				)
			},
		},
		{
			ID: "m20260122000000", // add attribute_overrides table for UI-configurable SMART overrides
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&m20260122000000.AttributeOverride{})
			},
		},
		{
			ID: "m20260124000000", // add missed ping notification settings
			Migrate: func(tx *gorm.DB) error {
				// Add missed ping notification settings with defaults
				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "metrics.notify_on_missed_ping",
						SettingKeyDescription: "Enable notifications when collectors miss their scheduled pings (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      false,
					},
					{
						SettingKeyName:        "metrics.missed_ping_timeout_minutes",
						SettingKeyDescription: "Minutes to wait before considering a collector missed (default: 60)",
						SettingDataType:       "numeric",
						SettingValueNumeric:   60,
					},
					{
						SettingKeyName:        "metrics.missed_ping_check_interval_mins",
						SettingKeyDescription: "How often to check for missed pings in minutes (default: 5)",
						SettingDataType:       "numeric",
						SettingValueNumeric:   5,
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
		{
			ID: "m20260129000000", // add smart_display_mode to device data
			Migrate: func(tx *gorm.DB) error {
				// adding column (smart_display_mode)
				return tx.AutoMigrate(m20260129000000.Device{})
			},
		},
		{
			ID: "m20260131000000", // add has_forced_failure to device data
			Migrate: func(tx *gorm.DB) error {
				// adding column (has_forced_failure)
				return tx.AutoMigrate(m20260131000000.Device{})
			},
		},
		{
			ID: "m20260202000000", // add collector_version to device data
			Migrate: func(tx *gorm.DB) error {
				// adding column (collector_version)
				return tx.AutoMigrate(m20260202000000.Device{})
			},
		},
		{
			ID: "m20260207000000", // add heartbeat notification settings
			Migrate: func(tx *gorm.DB) error {
				// Add heartbeat notification settings with defaults
				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "metrics.heartbeat_enabled",
						SettingKeyDescription: "Enable periodic heartbeat notifications when all drives are healthy (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      false,
					},
					{
						SettingKeyName:        "metrics.heartbeat_interval_hours",
						SettingKeyDescription: "Hours between heartbeat notifications (default: 24)",
						SettingDataType:       "numeric",
						SettingValueNumeric:   24,
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
		{
			ID: "m20260217000000", // add scheduled report settings
			Migrate: func(tx *gorm.DB) error {
				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "metrics.report_enabled",
						SettingKeyDescription: "Enable scheduled health reports (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      false,
					},
					{
						SettingKeyName:        "metrics.report_daily_enabled",
						SettingKeyDescription: "Enable daily reports (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      false,
					},
					{
						SettingKeyName:        "metrics.report_daily_time",
						SettingKeyDescription: "Time of day for daily report in 24h format (default: 08:00)",
						SettingDataType:       "string",
						SettingValueString:    "08:00",
					},
					{
						SettingKeyName:        "metrics.report_weekly_enabled",
						SettingKeyDescription: "Enable weekly reports (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      false,
					},
					{
						SettingKeyName:        "metrics.report_weekly_day",
						SettingKeyDescription: "Day of week for weekly report (0=Sunday, 1=Monday, default: 1)",
						SettingDataType:       "numeric",
						SettingValueNumeric:   1,
					},
					{
						SettingKeyName:        "metrics.report_weekly_time",
						SettingKeyDescription: "Time of day for weekly report in 24h format (default: 08:00)",
						SettingDataType:       "string",
						SettingValueString:    "08:00",
					},
					{
						SettingKeyName:        "metrics.report_monthly_enabled",
						SettingKeyDescription: "Enable monthly reports (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      false,
					},
					{
						SettingKeyName:        "metrics.report_monthly_day",
						SettingKeyDescription: "Day of month for monthly report (1-28, default: 1)",
						SettingDataType:       "numeric",
						SettingValueNumeric:   1,
					},
					{
						SettingKeyName:        "metrics.report_monthly_time",
						SettingKeyDescription: "Time of day for monthly report in 24h format (default: 08:00)",
						SettingDataType:       "string",
						SettingValueString:    "08:00",
					},
					{
						SettingKeyName:        "metrics.report_pdf_enabled",
						SettingKeyDescription: "Generate PDF alongside notification report (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      false,
					},
					{
						SettingKeyName:        "metrics.report_pdf_path",
						SettingKeyDescription: "Directory to save PDF reports (default: /opt/scrutiny/reports)",
						SettingDataType:       "string",
						SettingValueString:    "/opt/scrutiny/reports",
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
		{
			ID: "m20260219000000", // add scheduler last-run timestamp settings
			Migrate: func(tx *gorm.DB) error {
				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "metrics.report_last_daily_run",
						SettingKeyDescription: "Timestamp of last daily report run (internal, do not edit)",
						SettingDataType:       "string",
						SettingValueString:    "",
					},
					{
						SettingKeyName:        "metrics.report_last_weekly_run",
						SettingKeyDescription: "Timestamp of last weekly report run (internal, do not edit)",
						SettingDataType:       "string",
						SettingValueString:    "",
					},
					{
						SettingKeyName:        "metrics.report_last_monthly_run",
						SettingKeyDescription: "Timestamp of last monthly report run (internal, do not edit)",
						SettingDataType:       "string",
						SettingValueString:    "",
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
		{
			ID: "m20260225000000", // add api_tokens table for authentication
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&m20260225000000.ApiToken{})
			},
		},
		{
			ID: "m20260226000000", // add notify_urls table for UI-configurable notification endpoints
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&m20260226000000.NotifyUrl{})
			},
		},
		{
			ID: "m20260301000000", // add notification cooldown, quiet hours, and per-device timeout override
			Migrate: func(tx *gorm.DB) error {
				// Add missed_ping_timeout_override column to devices table
				if err := tx.AutoMigrate(m20260301000000.Device{}); err != nil {
					return err
				}

				// Add notification cooldown, rate limiting, and quiet hours settings
				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "metrics.missed_ping_cooldown_minutes",
						SettingKeyDescription: "Minutes between repeated missed ping notifications for the same device (0 = use timeout value)",
						SettingDataType:       "numeric",
						SettingValueNumeric:   0,
					},
					{
						SettingKeyName:        "metrics.notification_rate_limit",
						SettingKeyDescription: "Maximum notifications per hour across all types (0 = unlimited)",
						SettingDataType:       "numeric",
						SettingValueNumeric:   0,
					},
					{
						SettingKeyName:        "metrics.notification_quiet_start",
						SettingKeyDescription: "Time to stop sending notifications in HH:MM 24h format (empty = disabled)",
						SettingDataType:       "string",
						SettingValueString:    "",
					},
					{
						SettingKeyName:        "metrics.notification_quiet_end",
						SettingKeyDescription: "Time to resume sending notifications in HH:MM 24h format (empty = disabled)",
						SettingDataType:       "string",
						SettingValueString:    "",
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
		{
			ID: "m20260315000000", // add device_id (UUIDv5) column and backfill existing devices
			Migrate: func(tx *gorm.DB) error {
				// Add device_id column to devices table
				if err := tx.AutoMigrate(m20260315000000.Device{}); err != nil {
					return err
				}

				// Backfill: compute device_id for all existing devices
				var devices []struct {
					WWN          string
					ModelName    string
					SerialNumber string
				}
				if err := tx.Raw("SELECT wwn, model_name, serial_number FROM devices").Scan(&devices).Error; err != nil {
					return fmt.Errorf("could not query devices for backfill: %w", err)
				}

				for _, dev := range devices {
					id := deviceid.Generate(dev.ModelName, dev.SerialNumber, dev.WWN)
					if err := tx.Exec("UPDATE devices SET device_id = ? WHERE wwn = ?", id, dev.WWN).Error; err != nil {
						return fmt.Errorf("could not backfill device_id for %s: %w", dev.WWN, err)
					}
				}

				return nil
			},
		},
		{
			ID: "m20260401000000", // swap primary key from wwn to device_id
			Migrate: func(tx *gorm.DB) error {
				// Safety: ensure every device has a device_id before proceeding.
				var nullCount int64
				if err := tx.Raw("SELECT COUNT(*) FROM devices WHERE device_id IS NULL OR device_id = ''").Scan(&nullCount).Error; err != nil {
					return fmt.Errorf("could not check for null device_ids: %w", err)
				}
				if nullCount > 0 {
					return fmt.Errorf("found %d devices with NULL/empty device_id; run m20260315000000 backfill first", nullCount)
				}

				// Step 1: Create new table with device_id as PRIMARY KEY
				createSQL := `CREATE TABLE devices_new (
					device_id TEXT PRIMARY KEY,
					wwn TEXT,
					created_at DATETIME,
					updated_at DATETIME,
					deleted_at DATETIME,
					device_name TEXT,
					device_uuid TEXT,
					device_serial_id TEXT,
					device_label TEXT,
					manufacturer TEXT,
					model_name TEXT,
					interface_type TEXT,
					interface_speed TEXT,
					serial_number TEXT,
					firmware TEXT,
					rotation_speed INTEGER,
					capacity INTEGER,
					form_factor TEXT,
					smart_support NUMERIC,
					device_protocol TEXT,
					device_type TEXT,
					label TEXT,
					host_id TEXT,
					collector_version TEXT,
					smart_display_mode TEXT DEFAULT 'scrutiny',
					device_status INTEGER,
					has_forced_failure NUMERIC DEFAULT 0,
					archived NUMERIC,
					muted NUMERIC,
					missed_ping_timeout_override INTEGER DEFAULT 0
				)`
				if err := tx.Exec(createSQL).Error; err != nil {
					return fmt.Errorf("failed to create devices_new: %w", err)
				}

				// Step 2: Copy all data from old table
				copySQL := `INSERT INTO devices_new (
					device_id, wwn, created_at, updated_at, deleted_at,
					device_name, device_uuid, device_serial_id, device_label,
					manufacturer, model_name, interface_type, interface_speed,
					serial_number, firmware, rotation_speed, capacity,
					form_factor, smart_support, device_protocol, device_type,
					label, host_id, collector_version, smart_display_mode,
					device_status, has_forced_failure, archived, muted,
					missed_ping_timeout_override
				) SELECT
					device_id, wwn, created_at, updated_at, deleted_at,
					device_name, device_uuid, device_serial_id, device_label,
					manufacturer, model_name, interface_type, interface_speed,
					serial_number, firmware, rotation_speed, capacity,
					form_factor, smart_support, device_protocol, device_type,
					label, host_id, collector_version, smart_display_mode,
					device_status, has_forced_failure, archived, muted,
					missed_ping_timeout_override
				FROM devices`
				if err := tx.Exec(copySQL).Error; err != nil {
					return fmt.Errorf("failed to copy data to devices_new: %w", err)
				}

				// Step 3: Drop old table
				if err := tx.Exec("DROP TABLE devices").Error; err != nil {
					return fmt.Errorf("failed to drop old devices table: %w", err)
				}

				// Step 4: Rename new table
				if err := tx.Exec("ALTER TABLE devices_new RENAME TO devices").Error; err != nil {
					return fmt.Errorf("failed to rename devices_new: %w", err)
				}

				// Step 5: Partial unique index on wwn (allows multiple empty/NULL values)
				if err := tx.Exec("CREATE UNIQUE INDEX idx_devices_wwn ON devices(wwn) WHERE wwn IS NOT NULL AND wwn != ''").Error; err != nil {
					return fmt.Errorf("failed to create wwn unique index: %w", err)
				}

				// Step 6: Index on deleted_at (GORM soft-delete convention)
				if err := tx.Exec("CREATE INDEX idx_devices_deleted_at ON devices(deleted_at)").Error; err != nil {
					return fmt.Errorf("failed to create deleted_at index: %w", err)
				}

				return nil
			},
		},
		// Drop unique constraint on wwn, replace with regular index.
		// The unique index caused INSERT failures for devices sharing the same
		// non-empty WWN (e.g. multiple disks reporting 0x0000000000000000).
		// Since device_id is now the primary key, wwn uniqueness is no longer needed.
		// Fixes: https://github.com/Starosdev/scrutiny/issues/314
		{
			ID: "m20260402000000",
			Migrate: func(tx *gorm.DB) error {
				// Drop the unique index
				if err := tx.Exec("DROP INDEX IF EXISTS idx_devices_wwn").Error; err != nil {
					return fmt.Errorf("failed to drop unique wwn index: %w", err)
				}
				// Create a regular (non-unique) index for lookup performance
				if err := tx.Exec("CREATE INDEX idx_devices_wwn ON devices(wwn)").Error; err != nil {
					return fmt.Errorf("failed to create wwn index: %w", err)
				}
				return nil
			},
		},
		{
			ID: "m20260410000000", // add notify_on_collector_error setting
			Migrate: func(tx *gorm.DB) error {
				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "metrics.notify_on_collector_error",
						SettingKeyDescription: "Enable notifications when the collector encounters smartctl errors (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      true,
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
		{
			ID: "m20260411000000", // enforce unique constraint on (protocol, attribute_id, wwn) in attribute_overrides
			Migrate: func(tx *gorm.DB) error {
				// Remove any duplicate overrides, keeping the row with the lowest id
				// for each (protocol, attribute_id, wwn) combination.
				if err := tx.Exec(`
					DELETE FROM attribute_overrides
					WHERE id NOT IN (
						SELECT MIN(id)
						FROM attribute_overrides
						GROUP BY protocol, attribute_id, wwn
					)
				`).Error; err != nil {
					return fmt.Errorf("failed to remove duplicate attribute overrides: %w", err)
				}
				// Drop the existing non-unique composite index so we can replace it.
				if err := tx.Exec("DROP INDEX IF EXISTS idx_override_lookup").Error; err != nil {
					return fmt.Errorf("failed to drop old attribute_overrides index: %w", err)
				}
				// Create a unique composite index to prevent future duplicates.
				if err := tx.Exec("CREATE UNIQUE INDEX idx_override_lookup ON attribute_overrides (protocol, attribute_id, wwn)").Error; err != nil {
					return fmt.Errorf("failed to create unique attribute_overrides index: %w", err)
				}
				return nil
			},
		},
		{
			ID: "m20260413000000", // add Uptime Kuma push monitor settings (#351)
			Migrate: func(tx *gorm.DB) error {
				var defaultSettings = []m20220716214900.Setting{
					{
						SettingKeyName:        "metrics.uptime_kuma_enabled",
						SettingKeyDescription: "Enable Uptime Kuma push monitor (true | false)",
						SettingDataType:       "bool",
						SettingValueBool:      false,
					},
					{
						SettingKeyName:        "metrics.uptime_kuma_push_url",
						SettingKeyDescription: "Uptime Kuma push monitor URL",
						SettingDataType:       "string",
						SettingValueString:    "",
					},
					{
						SettingKeyName:        "metrics.uptime_kuma_interval_seconds",
						SettingKeyDescription: "Seconds between Uptime Kuma pushes (default: 60)",
						SettingDataType:       "numeric",
						SettingValueNumeric:   60,
					},
				}
				return tx.Create(&defaultSettings).Error
			},
		},
	})

	if err := m.Migrate(); err != nil {
		if strings.Contains(err.Error(), "readonly database") ||
			strings.Contains(err.Error(), "attempt to write") {
			sr.logger.Errorf("Database migration failed: unable to write to database.\n\n"+
				"This error commonly occurs in Docker containers with restricted capabilities.\n"+
				"Solutions:\n"+
				"1. Check file permissions on the database directory\n"+
				"2. If using 'cap_drop: [ALL]', add necessary capabilities back\n"+
				"3. Verify the volume mount has correct ownership\n\n"+
				"Original error: %v", err)
		} else {
			sr.logger.Errorf("Database migration failed with error.\nPlease open a github issue at https://github.com/Starosdev/scrutiny and attach a copy of your scrutiny.db file.\n%v", err)
		}
		return err
	}
	sr.logger.Infoln("Database migration completed successfully")

	//these migrations cannot be done within a transaction, so they are done as a separate group, with `UseTransaction = false`
	sr.logger.Infoln("SQLite global configuration migrations starting. Please wait....")
	globalMigrateOptions := gormigrate.DefaultOptions
	globalMigrateOptions.UseTransaction = false
	gm := gormigrate.New(sr.gormClient, globalMigrateOptions, []*gormigrate.Migration{
		{
			ID: "g20220802211500",
			Migrate: func(tx *gorm.DB) error {
				//shrink the Database (maybe necessary after 20220503113100)
				if err := tx.Exec("VACUUM;").Error; err != nil {
					return err
				}
				return nil
			},
		},
	})

	if err := gm.Migrate(); err != nil {
		if strings.Contains(err.Error(), "readonly database") ||
			strings.Contains(err.Error(), "attempt to write") {
			sr.logger.Errorf("SQLite global configuration migrations failed: unable to write to database.\n\n"+
				"This error commonly occurs in Docker containers with restricted capabilities.\n"+
				"Solutions:\n"+
				"1. Check file permissions on the database directory\n"+
				"2. If using 'cap_drop: [ALL]', add necessary capabilities back\n"+
				"3. Verify the volume mount has correct ownership\n\n"+
				"Original error: %v", err)
		} else {
			sr.logger.Errorf("SQLite global configuration migrations failed with error.\nPlease open a github issue at https://github.com/Starosdev/scrutiny and attach a copy of your scrutiny.db file.\n%v", err)
		}
		return err
	}
	sr.logger.Infoln("SQLite global configuration migrations completed successfully")

	return nil
}

// helpers

// When adding data to influxdb, an error may be returned if the data point is outside the range of the retention policy.
// This function will ignore retention policy errors, and allow the migration to continue.
func (sr *scrutinyRepository) ignorePastRetentionPolicyError(err error) error {
	var influxDbWriteError *http.Error
	if errors.As(err, &influxDbWriteError) {
		if influxDbWriteError.StatusCode == 422 {
			sr.logger.Infoln("ignoring error: attempted to writePoint past retention period duration")
			return nil
		}
	}
	return err
}

// Deprecated
func m20201107210306_FromPreInfluxDBTempCreatePostInfluxDBTemp(preDevice m20201107210306.Device, preSmartResult m20201107210306.Smart) (error, measurements.SmartTemperature) {
	//extract temperature data for every datapoint
	postSmartTemp := measurements.SmartTemperature{
		Date: preSmartResult.TestDate,
		Temp: preSmartResult.Temp,
	}

	return nil, postSmartTemp
}

// Deprecated
func m20201107210306_FromPreInfluxDBSmartResultsCreatePostInfluxDBSmartResults(database *gorm.DB, preDevice m20201107210306.Device, preSmartResult m20201107210306.Smart) (error, measurements.Smart) {
	//create a measurements.Smart object (which we will then push to the InfluxDB)
	postDeviceSmartData := measurements.Smart{
		Date:            preSmartResult.TestDate,
		DeviceWWN:       preDevice.WWN,
		DeviceProtocol:  preDevice.DeviceProtocol,
		Temp:            preSmartResult.Temp,
		PowerOnHours:    preSmartResult.PowerOnHours,
		PowerCycleCount: preSmartResult.PowerCycleCount,

		// this needs to be populated using measurements.Smart.ProcessAtaSmartInfo, ProcessScsiSmartInfo or ProcessNvmeSmartInfo
		// because those functions will take into account thresholds (which we didn't consider correctly previously)
		Attributes: map[string]measurements.SmartAttribute{},
	}

	result := database.Preload("AtaAttributes").Preload("NvmeAttributes").Preload("ScsiAttributes").Find(&preSmartResult)
	if result.Error != nil {
		return result.Error, postDeviceSmartData
	}

	if preDevice.IsAta() {
		preAtaSmartAttributesTable := []collector.AtaSmartAttributesTableItem{}
		for _, preAtaAttribute := range preSmartResult.AtaAttributes {
			preAtaSmartAttributesTable = append(preAtaSmartAttributesTable, collector.AtaSmartAttributesTableItem{
				ID:         preAtaAttribute.AttributeId,
				Name:       preAtaAttribute.Name,
				Value:      int64(preAtaAttribute.Value),
				Worst:      int64(preAtaAttribute.Worst),
				Thresh:     int64(preAtaAttribute.Threshold),
				WhenFailed: preAtaAttribute.WhenFailed,
				Flags: struct {
					Value         int    `json:"value"`
					String        string `json:"string"`
					Prefailure    bool   `json:"prefailure"`
					UpdatedOnline bool   `json:"updated_online"`
					Performance   bool   `json:"performance"`
					ErrorRate     bool   `json:"error_rate"`
					EventCount    bool   `json:"event_count"`
					AutoKeep      bool   `json:"auto_keep"`
				}{
					Value:         0,
					String:        "",
					Prefailure:    false,
					UpdatedOnline: false,
					Performance:   false,
					ErrorRate:     false,
					EventCount:    false,
					AutoKeep:      false,
				},
				Raw: struct {
					Value  int64  `json:"value"`
					String string `json:"string"`
				}{
					Value:  preAtaAttribute.RawValue,
					String: preAtaAttribute.RawString,
				},
			})
		}

		postDeviceSmartData.ProcessAtaSmartInfo(nil, preAtaSmartAttributesTable)

	} else if preDevice.IsNvme() {
		//info collector.SmartInfo
		postNvmeSmartHealthInformation := collector.NvmeSmartHealthInformationLog{}

		for _, preNvmeAttribute := range preSmartResult.NvmeAttributes {
			switch preNvmeAttribute.AttributeId {
			case "critical_warning":
				postNvmeSmartHealthInformation.CriticalWarning = int64(preNvmeAttribute.Value)
			case "temperature":
				postNvmeSmartHealthInformation.Temperature = int64(preNvmeAttribute.Value)
			case "available_spare":
				postNvmeSmartHealthInformation.AvailableSpare = int64(preNvmeAttribute.Value)
			case "available_spare_threshold":
				postNvmeSmartHealthInformation.AvailableSpareThreshold = int64(preNvmeAttribute.Value)
			case "percentage_used":
				postNvmeSmartHealthInformation.PercentageUsed = int64(preNvmeAttribute.Value)
			case "data_units_read":
				postNvmeSmartHealthInformation.DataUnitsWritten = int64(preNvmeAttribute.Value)
			case "data_units_written":
				postNvmeSmartHealthInformation.DataUnitsWritten = int64(preNvmeAttribute.Value)
			case "host_reads":
				postNvmeSmartHealthInformation.HostReads = int64(preNvmeAttribute.Value)
			case "host_writes":
				postNvmeSmartHealthInformation.HostWrites = int64(preNvmeAttribute.Value)
			case "controller_busy_time":
				postNvmeSmartHealthInformation.ControllerBusyTime = int64(preNvmeAttribute.Value)
			case "power_cycles":
				postNvmeSmartHealthInformation.PowerCycles = int64(preNvmeAttribute.Value)
			case "power_on_hours":
				postNvmeSmartHealthInformation.PowerOnHours = int64(preNvmeAttribute.Value)
			case "unsafe_shutdowns":
				postNvmeSmartHealthInformation.UnsafeShutdowns = int64(preNvmeAttribute.Value)
			case "media_errors":
				postNvmeSmartHealthInformation.MediaErrors = int64(preNvmeAttribute.Value)
			case "num_err_log_entries":
				postNvmeSmartHealthInformation.NumErrLogEntries = int64(preNvmeAttribute.Value)
			case "warning_temp_time":
				postNvmeSmartHealthInformation.WarningTempTime = int64(preNvmeAttribute.Value)
			case "critical_comp_time":
				postNvmeSmartHealthInformation.CriticalCompTime = int64(preNvmeAttribute.Value)
			}
		}

		postDeviceSmartData.ProcessNvmeSmartInfo(nil, postNvmeSmartHealthInformation)

	} else if preDevice.IsScsi() {
		//info collector.SmartInfo
		var postScsiGrownDefectList int64
		postScsiErrorCounterLog := collector.ScsiErrorCounterLog{
			Read: struct {
				ErrorsCorrectedByEccfast         int64  `json:"errors_corrected_by_eccfast"`
				ErrorsCorrectedByEccdelayed      int64  `json:"errors_corrected_by_eccdelayed"`
				ErrorsCorrectedByRereadsRewrites int64  `json:"errors_corrected_by_rereads_rewrites"`
				TotalErrorsCorrected             int64  `json:"total_errors_corrected"`
				CorrectionAlgorithmInvocations   int64  `json:"correction_algorithm_invocations"`
				GigabytesProcessed               string `json:"gigabytes_processed"`
				TotalUncorrectedErrors           int64  `json:"total_uncorrected_errors"`
			}{},
			Write: struct {
				ErrorsCorrectedByEccfast         int64  `json:"errors_corrected_by_eccfast"`
				ErrorsCorrectedByEccdelayed      int64  `json:"errors_corrected_by_eccdelayed"`
				ErrorsCorrectedByRereadsRewrites int64  `json:"errors_corrected_by_rereads_rewrites"`
				TotalErrorsCorrected             int64  `json:"total_errors_corrected"`
				CorrectionAlgorithmInvocations   int64  `json:"correction_algorithm_invocations"`
				GigabytesProcessed               string `json:"gigabytes_processed"`
				TotalUncorrectedErrors           int64  `json:"total_uncorrected_errors"`
			}{},
		}

		for _, preScsiAttribute := range preSmartResult.ScsiAttributes {
			switch preScsiAttribute.AttributeId {
			case "scsi_grown_defect_list":
				postScsiGrownDefectList = int64(preScsiAttribute.Value)
			case "read.errors_corrected_by_eccfast":
				postScsiErrorCounterLog.Read.ErrorsCorrectedByEccfast = int64(preScsiAttribute.Value)
			case "read.errors_corrected_by_eccdelayed":
				postScsiErrorCounterLog.Read.ErrorsCorrectedByEccdelayed = int64(preScsiAttribute.Value)
			case "read.errors_corrected_by_rereads_rewrites":
				postScsiErrorCounterLog.Read.ErrorsCorrectedByRereadsRewrites = int64(preScsiAttribute.Value)
			case "read.total_errors_corrected":
				postScsiErrorCounterLog.Read.TotalErrorsCorrected = int64(preScsiAttribute.Value)
			case "read.correction_algorithm_invocations":
				postScsiErrorCounterLog.Read.CorrectionAlgorithmInvocations = int64(preScsiAttribute.Value)
			case "read.total_uncorrected_errors":
				postScsiErrorCounterLog.Read.TotalUncorrectedErrors = int64(preScsiAttribute.Value)
			case "write.errors_corrected_by_eccfast":
				postScsiErrorCounterLog.Write.ErrorsCorrectedByEccfast = int64(preScsiAttribute.Value)
			case "write.errors_corrected_by_eccdelayed":
				postScsiErrorCounterLog.Write.ErrorsCorrectedByEccdelayed = int64(preScsiAttribute.Value)
			case "write.errors_corrected_by_rereads_rewrites":
				postScsiErrorCounterLog.Write.ErrorsCorrectedByRereadsRewrites = int64(preScsiAttribute.Value)
			case "write.total_errors_corrected":
				postScsiErrorCounterLog.Write.TotalErrorsCorrected = int64(preScsiAttribute.Value)
			case "write.correction_algorithm_invocations":
				postScsiErrorCounterLog.Write.CorrectionAlgorithmInvocations = int64(preScsiAttribute.Value)
			case "write.total_uncorrected_errors":
				postScsiErrorCounterLog.Write.TotalUncorrectedErrors = int64(preScsiAttribute.Value)
			}
		}
		postDeviceSmartData.ProcessScsiSmartInfo(nil, postScsiGrownDefectList, postScsiErrorCounterLog, nil)
	} else {
		return fmt.Errorf("Unknown device protocol: %s", preDevice.DeviceProtocol), postDeviceSmartData
	}

	return nil, postDeviceSmartData
}
