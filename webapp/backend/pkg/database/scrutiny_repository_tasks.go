package database

import (
	"context"
	"fmt"
	"sort"

	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// //////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Tasks
// //////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func (sr *scrutinyRepository) EnsureTasks(ctx context.Context, orgID string) error {
	weeklyTaskName := "tsk-weekly-aggr"
	// weekly on Sunday at 1:00am
	weeklyTaskScript := sr.DownsampleScript("weekly", weeklyTaskName, "0 1 * * 0")
	if err := sr.ensureDownsampleTask(ctx, orgID, weeklyTaskName, weeklyTaskScript, "weekly"); err != nil {
		return err
	}

	monthlyTaskName := "tsk-monthly-aggr"
	// monthly on first day of the month at 1:30am
	monthlyTaskScript := sr.DownsampleScript("monthly", monthlyTaskName, "30 1 1 * *")
	if err := sr.ensureDownsampleTask(ctx, orgID, monthlyTaskName, monthlyTaskScript, "monthly"); err != nil {
		return err
	}

	yearlyTaskName := "tsk-yearly-aggr"
	// yearly on the first day of the year at 2:00am
	yearlyTaskScript := sr.DownsampleScript("yearly", yearlyTaskName, "0 2 1 1 *")
	if err := sr.ensureDownsampleTask(ctx, orgID, yearlyTaskName, yearlyTaskScript, "yearly"); err != nil {
		return err
	}
	return nil
}

// ensureDownsampleTask converges on exactly one task named taskName in orgID, running the
// current flux script.
//
// Task names are not unique in InfluxDB, so this check-then-create races the same way bucket
// creation does (see ensureBucketExists): two processes starting together both find no task and
// both create one, and InfluxDB reports no conflict. Each copy is scheduled and writes to the
// same destination bucket, so duplicates are pruned rather than tolerated, and the keeper is
// chosen by lowest ID so that two processes pruning at once converge on the same task instead
// of deleting each other's.
func (sr *scrutinyRepository) ensureDownsampleTask(ctx context.Context, orgID, taskName, taskScript, label string) error {
	found, findErr := sr.findDownsampleTasks(ctx, orgID, taskName, label)
	if findErr != nil {
		return nil
	}

	if len(found) == 0 {
		if _, err := sr.influxTaskApi.CreateTaskByFlux(ctx, taskScript, orgID); err != nil {
			return err
		}
		// a concurrent process may have created the same task: look again and prune now, rather
		// than leaving the duplicate scheduled until some later startup
		if found, findErr = sr.findDownsampleTasks(ctx, orgID, taskName, label); findErr != nil {
			return nil
		}
	}
	if len(found) == 0 {
		return nil
	}

	sort.Slice(found, func(i, j int) bool { return found[i].Id < found[j].Id })
	keeper := &found[0]
	if taskScript != keeper.Flux {
		sr.logger.Infoln("updating " + label + " task script")
		keeper.Flux = taskScript
		if _, err := sr.influxTaskApi.UpdateTask(ctx, keeper); err != nil {
			return err
		}
	}

	for _, duplicate := range found[1:] {
		sr.logger.Warnf("deleting duplicate %s downsample task %s", label, duplicate.Id)
		if err := sr.influxTaskApi.DeleteTaskWithID(ctx, duplicate.Id); err != nil {
			// another process pruning the same duplicate gets here second, and a duplicate that
			// is already gone is the wanted end state
			sr.logger.Warnf("could not delete duplicate %s downsample task %s: %s", label, duplicate.Id, err.Error())
		}
	}
	return nil
}

// findDownsampleTasks lists the tasks named taskName in orgID. The filter is scoped to the
// organization because task names are unique to neither it nor the task, and an unscoped query
// matches by name across every organization the token can read.
func (sr *scrutinyRepository) findDownsampleTasks(ctx context.Context, orgID, taskName, label string) ([]domain.Task, error) {
	found, err := sr.influxTaskApi.FindTasks(ctx, &api.TaskFilter{Name: taskName, OrgID: orgID})
	if err != nil {
		// a lookup failure leaves the task alone, as it always has, but it is no longer silent
		sr.logger.Warnf("could not look up the %s downsample task: %s", label, err.Error())
		return nil, err
	}
	return found, nil
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

	// Aggregates group by device_id as well as device_wwn, so devices that share a WWN are
	// aggregated separately (#851). Grouping by WWN alone merged them: the smart "last" selector
	// kept the device_id of whichever row came last, and the temperature mean dropped device_id.
	// ensureDownsampleTask replaces existing task scripts when this text changes.
	//
	// TODO: using "last" function for aggregation. This should eventually be replaced with a more accurate represenation
	/*
	  import "types"
	  smart_data = from(bucket: sourceBucket)
	  |> range(start: rangeStart, stop: rangeEnd)
	  |> filter(fn: (r) => r["_measurement"] == "smart" )
	  |> group(columns: ["device_id", "device_wwn", "_field"])

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
|> group(columns: ["device_id", "device_wwn", "_field"])
|> aggregateWindow(every: aggWindow, fn: last, createEmpty: false)
|> to(bucket: destBucket, org: destOrg)

from(bucket: sourceBucket)
|> range(start: rangeStart, stop: rangeEnd)
|> filter(fn: (r) => r["_measurement"] == "temp")
|> group(columns: ["device_id", "device_wwn"])
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
