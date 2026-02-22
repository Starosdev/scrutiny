# Downsampling

Scrutiny collects alot of data, that can cause the database to grow unbounded. 

- Smart data
- Smart test data
- Temperature data
- Disk metrics (capacity/usage)
- etc

This data must be accurate in the short term, and is useful for doing trend analysis in the long term.
However, for trend analysis we only need aggregate data, individual data points are not as useful.

Scrutiny will automatically downsample data on a schedule to ensure that the database size stays reasonable, while still
ensuring historical data is present for comparisons.


| Bucket Name | Retention Period | Downsampling Range | Downsampling Aggregation Window | Downsampling Cron | Comments |
| --- | --- | --- | --- | --- | --- |
| `metrics` | 15 days | `-2w -1w` | `1w` | main bucket, weekly on Sunday at 1:00am |
| `metrics_weekly` | 9 weeks | `-2mo -1mo` | `1mo` | monthly on first day of the month at 1:30am
| `metrics_monthly` | 25 months | `-2y -1y` | `1y` | yearly on the first day of the year at 2:00am
| `metrics_yearly` | forever | - | - | - | |


After 5 months, here's how may data points should exist in each bucket for one disk

| Bucket Name | Datapoints | Comments |
| --- | --- | --- |
| `metrics` | 15 | 7 daily datapoints , up to 7 pending data, 1 buffer data point |
| `metrics_weekly` | 9 | 4 aggregated weekly data points, 4 pending datapoints, 1 buffer data point |
| `metrics_monthly` | 3 | 3 aggregated monthly data points | 
| `metrics_yearly` | 0 | |

After 5 years, here's how may data points should exist in each bucket for one disk

| Bucket Name | Datapoints | Comments |
| --- | --- | --- |
| `metrics` | - | - |
| `metrics_weekly` | - | 
| `metrics_monthly` | - |
| `metrics_yearly` | - |

## Workload Insights and Downsampled Data

The Workload Insights page (`/api/summary/workload`) computes daily read/write rates by querying cumulative SMART counters (e.g., Total LBAs Written, Data Units Written) across multiple buckets. It uses the same multi-bucket union query pattern as temperature history, selecting the first and last data points in the requested time range.

All SMART fields use `fn: last` in downsampling tasks, which means cumulative counters are preserved correctly -- the last value before each aggregation window is retained.

**Zero-filled entry filtering:** Downsampled buckets can contain entries where cumulative counter fields are null or zero (e.g., from before a device started reporting a particular attribute). The workload query filters these out to prevent using a zero-valued "first" point, which would make the delta equal to the device's entire lifetime of writes and grossly inflate daily rate calculations.

