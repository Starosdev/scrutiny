package main

import (
	"context"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"os"
)

func main() {
    // We can just query localhost:8086 if it's there
	token := os.Getenv("INFLUXDB_TOKEN")
	if token == "" {
		token = "my-super-secret-auth-token" // Standard dev token often used for docker local tests
	}
	client := influxdb2.NewClient("http://192.168.26.111:8086", "scrutiny-token")
	queryAPI := client.QueryAPI("scrutiny")
	
	query := `from(bucket: "metrics") |> range(start: -7d) |> filter(fn: (r) => r["_measurement"] == "mdadm_array")`
	
	result, err := queryAPI.Query(context.Background(), query)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	count := 0
	for result.Next() {
		count++
		if count <= 5 {
		    fmt.Printf("Record %d: Measurement: %s, Field: %s, Value: %v\n", count, result.Record().Measurement(), result.Record().Field(), result.Record().Value())
		    for k, v := range result.Record().Values() {
		        fmt.Printf("  %s: %v\n", k, v)
		    }
		}
	}
	fmt.Printf("Total records: %d\n", count)
	if result.Err() != nil {
		fmt.Printf("Query error: %v\n", result.Err())
	}
}
