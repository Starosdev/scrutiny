package database

import (
	"context"
	"fmt"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

// //////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Tasks
// //////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func (sr *scrutinyRepository) EnsureTasks(ctx context.Context, orgID string) error {
	weeklyTaskName := "tsk-weekly-aggr"
	//weekly on Sunday at 1:00am
	weeklyTaskScript := sr.DownsampleScript("weekly", weeklyTaskName, "0 1 * * 0")
	if err := sr.ensureDownsampleTask(ctx, orgID, weeklyTaskName, weeklyTaskScript, "weekly"); err != nil {
		return err
	}

	monthlyTaskName := "tsk-monthly-aggr"
	//monthly on first day of the month at 1:30am
	monthlyTaskScript := sr.DownsampleScript("monthly", monthlyTaskName, "30 1 1 * *")
	if err := sr.ensureDownsampleTask(ctx, orgID, monthlyTaskName, monthlyTaskScript, "monthly"); err != nil {
		return err
	}

	yearlyTaskName := "tsk-yearly-aggr"
	//yearly on the first day of the year at 2:00am
	yearlyTaskScript := sr.DownsampleScript("yearly", yearlyTaskName, "0 2 1 1 *")
	if err := sr.ensureDownsampleTask(ctx, orgID, yearlyTaskName, yearlyTaskScript, "yearly"); err != nil {
		return err
	}
	return nil
}

// ensureDownsampleTask creates the named downsample task when it does not exist,
// or updates its flux script when the single existing task differs.
func (sr *scrutinyRepository) ensureDownsampleTask(ctx context.Context, orgID, taskName, taskScript, label string) error {
	found, findErr := sr.influxTaskApi.FindTasks(ctx, &api.TaskFilter{Name: taskName})
	if findErr == nil && len(found) == 0 {
		_, err := sr.influxTaskApi.CreateTaskByFlux(ctx, taskScript, orgID)
		return err
	} else if len(found) == 1 {
		//check if we should update
		task := &found[0]
		if taskScript != task.Flux {
			sr.logger.Infoln("updating " + label + " task script")
			task.Flux = taskScript
			_, err := sr.influxTaskApi.UpdateTask(ctx, task)
			return err
		}
	}
	return nil
}

func (sr *scrutinyRepository) DownsampleScript(aggregationType string, name string, cron string) string {
	var sourceBucket string // the source of the data
	var destBucket string   // the destination for the aggregated data
	var rangeStart string
	var rangeEnd string
	var aggWindow string
	switch aggregationType {
	case "weekly":
		sourceBucket = sr.appConfig.GetString(cfgInfluxDBBucket)
		destBucket = fmt.Sprintf("%s_weekly", sr.appConfig.GetString(cfgInfluxDBBucket))
		rangeStart = "-2w"
		rangeEnd = "-1w"
		aggWindow = "1w"
	case "monthly":
		sourceBucket = fmt.Sprintf("%s_weekly", sr.appConfig.GetString(cfgInfluxDBBucket))
		destBucket = fmt.Sprintf("%s_monthly", sr.appConfig.GetString(cfgInfluxDBBucket))
		rangeStart = "-2mo"
		rangeEnd = "-1mo"
		aggWindow = "1mo"
	case "yearly":
		sourceBucket = fmt.Sprintf("%s_monthly", sr.appConfig.GetString(cfgInfluxDBBucket))
		destBucket = fmt.Sprintf("%s_yearly", sr.appConfig.GetString(cfgInfluxDBBucket))
		rangeStart = "-2y"
		rangeEnd = "-1y"
		aggWindow = "1y"
	}

	// TODO: using "last" function for aggregation. This should eventually be replaced with a more accurate represenation
	/*
	  import "types"
	  smart_data = from(bucket: sourceBucket)
	  |> range(start: rangeStart, stop: rangeEnd)
	  |> filter(fn: (r) => r["_measurement"] == "smart" )
	  |> group(columns: ["device_wwn", "_field"])

	  non_numeric_smart_data = smart_data
	    |> filter(fn: (r) => types.isType(v: r._value, type: "string") or types.isType(v: r._value, type: "bool"))
	    |> aggregateWindow(every: aggWindow, fn: last, createEmpty: false)

	  numeric_smart_data = smart_data
	    |> filter(fn: (r) => types.isType(v: r._value, type: "int") or types.isType(v: r._value, type: "float"))
	    |> aggregateWindow(every: aggWindow, fn: mean, createEmpty: false)

	  union(tables: [non_numeric_smart_data, numeric_smart_data])
	  |> to(bucket: destBucket, org: destOrg)

	*/

	return fmt.Sprintf(`
option task = { 
  name: "%s",
  cron: "%s",
}

sourceBucket = "%s"
rangeStart = %s
rangeEnd = %s
aggWindow = %s
destBucket = "%s"
destOrg = "%s"

from(bucket: sourceBucket)
|> range(start: rangeStart, stop: rangeEnd)
|> filter(fn: (r) => r["_measurement"] == "smart" )
|> group(columns: ["device_wwn", "_field"])
|> aggregateWindow(every: aggWindow, fn: last, createEmpty: false)
|> to(bucket: destBucket, org: destOrg)

from(bucket: sourceBucket)
|> range(start: rangeStart, stop: rangeEnd)
|> filter(fn: (r) => r["_measurement"] == "temp")
|> group(columns: ["device_wwn"])
|> toInt()
|> aggregateWindow(fn: mean, every: aggWindow, createEmpty: false)
|> set(key: "_measurement", value: "temp")
|> set(key: "_field", value: "temp")
|> to(bucket: destBucket, org: destOrg)
		`,
		name,
		cron,
		sourceBucket,
		rangeStart,
		rangeEnd,
		aggWindow,
		destBucket,
		sr.appConfig.GetString(cfgInfluxDBOrg),
	)
}
