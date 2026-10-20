package database

import (
	"context"
	"fmt"
	"sort"
	"strings"

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

// ensureDownsampleTask converges on exactly one task named taskName in orgID that writes to
// this deployment's destination bucket, running the current flux script.
//
// Task names are not unique in InfluxDB, so this check-then-create races the same way bucket
// creation does (see ensureBucketExists): two processes starting together both find no task and
// both create one, and InfluxDB reports no conflict. Each copy is scheduled and writes to the
// same destination bucket, so duplicates are pruned rather than tolerated, and the keeper is
// chosen by lowest ID so that two processes pruning at once converge on the same task instead
// of deleting each other's.
//
// Only tasks writing to this deployment's destination bucket are touched. The task names are
// fixed strings rather than derived from the configured bucket, so a second Scrutiny deployment
// sharing this organization with a different web.influxdb.bucket owns same-named tasks whose
// flux differs, and those are left alone.
func (sr *scrutinyRepository) ensureDownsampleTask(ctx context.Context, orgID, taskName, taskScript, label string) error {
	found, findErr := sr.findDownsampleTasks(ctx, orgID, taskName, label)
	if findErr != nil {
		return nil
	}

	owned, decidable := sr.ownedDownsampleTasks(found, taskScript, label)
	if decidable && len(owned) == 0 {
		if _, err := sr.influxTaskApi.CreateTaskByFlux(ctx, taskScript, orgID); err != nil {
			return err
		}
		// a concurrent process may have created the same task: look again and prune now, rather
		// than leaving the duplicate scheduled until some later startup
		if found, findErr = sr.findDownsampleTasks(ctx, orgID, taskName, label); findErr != nil {
			return nil
		}
		owned, _ = sr.ownedDownsampleTasks(found, taskScript, label)
	}
	if len(owned) == 0 {
		return nil
	}

	sort.Slice(owned, func(i, j int) bool { return owned[i].Id < owned[j].Id })
	keeper := owned[0]
	if taskScript != keeper.Flux {
		sr.logger.Infoln("updating " + label + " task script")
		keeper.Flux = taskScript
		if _, err := sr.influxTaskApi.UpdateTask(ctx, keeper); err != nil {
			return err
		}
	}

	for _, duplicate := range owned[1:] {
		sr.logger.Warnf("deleting duplicate %s downsample task %s", label, duplicate.Id)
		if err := sr.influxTaskApi.DeleteTaskWithID(ctx, duplicate.Id); err != nil {
			// another process pruning the same duplicate gets here second, and a duplicate that
			// is already gone is the wanted end state
			sr.logger.Warnf("could not delete duplicate %s downsample task %s: %s", label, duplicate.Id, err.Error())
		}
	}
	return nil
}

// ownedDownsampleTasks returns the tasks among found that write to the same destination bucket as
// taskScript, which is what makes them this deployment's to update and to delete.
//
// Tasks are matched on the generated script's own destBucket assignment, so the marker cannot
// drift from what DownsampleScript emits. A generated script without that line is a defect in
// the template rather than a deployment state, and the second return value reports that
// ownership could not be decided: the caller then leaves whatever is there alone instead of
// deleting a task, or creating another, on a guess. A lone task is still updated, which is what
// the code did before pruning existed.
func (sr *scrutinyRepository) ownedDownsampleTasks(found []domain.Task, taskScript, label string) ([]*domain.Task, bool) {
	marker := downsampleDestBucketLine(taskScript)
	if marker == "" {
		sr.logger.Errorf("the generated %s downsample script carries no destBucket assignment", label)
		if len(found) == 1 {
			return []*domain.Task{&found[0]}, false
		}
		return nil, false
	}

	owned := make([]*domain.Task, 0, len(found))
	for i := range found {
		if strings.Contains(found[i].Flux, marker) {
			owned = append(owned, &found[i])
		}
	}
	if len(owned) < len(found) {
		sr.logger.Warnf("leaving %d %s task(s) that write to another destination bucket in place", len(found)-len(owned), label)
	}
	return owned, true
}

// downsampleDestBucketLine returns the `destBucket = "..."` assignment from a generated
// downsample script, or an empty string when the script does not carry one.
func downsampleDestBucketLine(taskScript string) string {
	for _, line := range strings.Split(taskScript, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, `destBucket = "`) {
			return trimmed
		}
	}
	return ""
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
