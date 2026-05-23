package main

import (
	"fmt"
	"os"

	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
)

func main() {
	const path = "webapp/backend/pkg/thresholds/consumer_drive_profiles.json"

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}

	if err := thresholds.ValidateConsumerDriveProfileCatalog(data); err != nil {
		fmt.Fprintf(os.Stderr, "invalid consumer drive profile catalog: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("consumer drive profile catalog is valid")
}
