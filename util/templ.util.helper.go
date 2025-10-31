package util

import (
	"fmt"

	"github.com/MishraShardendu22/Scanner/models"
)

func CountFindings(result models.SCAN_RESULT) int {
	count := 0
	for _, resource := range result.ScannedResources {
		count += len(resource.Findings)
	}
	return count
}

func CountTotalResources(results []models.SCAN_RESULT) int {
	count := 0
	for _, result := range results {
		count += len(result.ScannedResources)
	}
	return count
}

func CountTotalFindingsInList(results []models.SCAN_RESULT) int {
	count := 0
	for _, result := range results {
		count += CountFindings(result)
	}
	return count
}

func GetResourceTypes(result models.SCAN_RESULT) string {
	typeMap := make(map[string]bool)
	for _, resource := range result.ScannedResources {
		typeMap[resource.Type] = true
	}
	return fmt.Sprintf("%d", len(typeMap))
}

func GetCurrentTime() string {
	return "Just now"
}

