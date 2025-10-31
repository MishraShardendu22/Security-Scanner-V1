package controller

import (
	"log"
	"time"

	"github.com/MishraShardendu22/Scanner/models"
	"github.com/MishraShardendu22/Scanner/util"
	"github.com/gofiber/fiber/v2"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/google/uuid"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

// dashboard data ke liye
// severity of secrets leaked
func GetDashboard(c *fiber.Ctx) error {
	start := time.Now()
	requestID := uuid.New().String()

	log.Printf(
		"op=GetDashboard stage=start request_id=%s method=%s path=%s ip=%s user_agent=%q",
		requestID, c.Method(), c.OriginalURL(), c.IP(), c.Get("User-Agent"),
	)

	scanResults := []models.SCAN_RESULT{}
	err := mgm.Coll(&models.SCAN_RESULT{}).SimpleFind(&scanResults, bson.M{})
	if err != nil {
		log.Printf(
			"op=GetDashboard stage=db_query_error request_id=%s error=%v elapsed=%s",
			requestID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to fetch scan results", nil, "")
	}

	dashboardData := map[string]interface{}{
		"total_scans":           len(scanResults),
		"total_findings":        0,
		"by_resource_type":      make(map[string]int),
		"by_secret_type":        make(map[string]int),
		"by_source_type":        make(map[string]int),
		"recent_scans":          []map[string]interface{}{},
		"high_risk_findings":    []models.Finding{},
		"resources_with_issues": 0,
	}

	totalFindings := 0
	byResourceType := make(map[string]int)
	bySecretType := make(map[string]int)
	bySourceType := make(map[string]int)
	resourcesWithIssues := 0

	highRiskPatterns := map[string]bool{
		"AWS Access Key ID":       true,
		"GitHub PAT":              true,
		"OpenAI / LLM API Key":    true,
		"Stripe Secret Key":       true,
		"Database URI with creds": true,
		"PostgreSQL URI":          true,
		"MySQL URI":               true,
		"MongoDB URI":             true,
		"Google API Key":          true,
		"Kubernetes Bearer Token": true,
		"GitHub Actions Token":    true,
	}

	highRiskFindings := []models.Finding{}
	recentScans := []map[string]interface{}{}

	for _, scan := range scanResults {
		scanFindings := 0
		for _, resource := range scan.ScannedResources {
			if len(resource.Findings) > 0 {
				resourcesWithIssues++
			}
			byResourceType[resource.Type] += len(resource.Findings)
			for _, finding := range resource.Findings {
				scanFindings++
				totalFindings++
				bySecretType[finding.SecretType]++
				bySourceType[finding.SourceType]++
				if highRiskPatterns[finding.SecretType] {
					highRiskFindings = append(highRiskFindings, finding)
				}
			}
		}
		if len(recentScans) < 10 {
			recentScans = append(recentScans, map[string]interface{}{
				"scan_id":    scan.ID.Hex(),
				"request_id": scan.RequestID,
				"findings":   scanFindings,
				"resources":  len(scan.ScannedResources),
				"created_at": scan.CreatedAt,
			})
		}
	}

	dashboardData["total_findings"] = totalFindings
	dashboardData["by_resource_type"] = byResourceType
	dashboardData["by_secret_type"] = bySecretType
	dashboardData["by_source_type"] = bySourceType
	dashboardData["resources_with_issues"] = resourcesWithIssues
	dashboardData["recent_scans"] = recentScans
	dashboardData["high_risk_findings"] = highRiskFindings
	dashboardData["high_risk_count"] = len(highRiskFindings)

	severityBreakdown := map[string]int{
		"high":   len(highRiskFindings),
		"medium": 0,
		"low":    0,
	}

	mediumRiskCount := 0
	lowRiskCount := 0
	for secretType, count := range bySecretType {
		if !highRiskPatterns[secretType] {
			if secretType == "API Key" || secretType == "Access Token" {
				mediumRiskCount += count
			} else {
				lowRiskCount += count
			}
		}
	}
	severityBreakdown["medium"] = mediumRiskCount
	severityBreakdown["low"] = lowRiskCount
	dashboardData["severity_breakdown"] = severityBreakdown

	log.Printf(
		"op=GetDashboard stage=success request_id=%s total_scans=%d total_findings=%d resources_with_issues=%d high_risk=%d elapsed=%s",
		requestID, len(scanResults), totalFindings, resourcesWithIssues, len(highRiskFindings), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Dashboard data retrieved successfully", dashboardData, "")
}

// sabhi scan results ka
// paginated list of scan results (ye forntend ke liye tha if we wanted to show there then)
func GetAllResults(c *fiber.Ctx) error {

	start := time.Now()
	requestID := uuid.New().String()

	// ye pura pagination kaise karna h uska logic hai
	page, limit := util.ParsePagination(c)
	skip := (page - 1) * limit

	log.Printf(
		"op=GetAllResults stage=start request_id=%s method=%s path=%s page=%d limit=%d ip=%s user_agent=%q",
		requestID, c.Method(), c.OriginalURL(), page, limit, c.IP(), c.Get("User-Agent"),
	)

	scanResults := []models.SCAN_RESULT{}
	err := mgm.Coll(&models.SCAN_RESULT{}).SimpleFind(&scanResults, bson.M{})
	if err != nil {
		log.Printf(
			"op=GetAllResults stage=db_query_error request_id=%s error=%v elapsed=%s",
			requestID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to fetch scan results", nil, "")
	}

	total := len(scanResults)
	totalPages := (total + limit - 1) / limit
	startIdx := skip
	endIdx := skip + limit

	if startIdx >= total {
		scanResults = []models.SCAN_RESULT{}
	} else {
		if endIdx > total {
			endIdx = total
		}
		scanResults = scanResults[startIdx:endIdx]
	}

	// get summary like what was fetched earlier if asked to show in detail do that
	results := []map[string]interface{}{}
	for _, scan := range scanResults {
		findingCount := 0
		for _, resource := range scan.ScannedResources {
			findingCount += len(resource.Findings)
		}
		results = append(results, map[string]interface{}{
			"scan_id":    scan.ID.Hex(),
			"request_id": scan.RequestID,
			"findings":   findingCount,
			"resources":  len(scan.ScannedResources),
			"created_at": scan.CreatedAt,
		})
	}

	log.Printf(
		"op=GetAllResults stage=success request_id=%s page=%d limit=%d total=%d total_pages=%d returned=%d elapsed=%s",
		requestID, page, limit, total, totalPages, len(results), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Scan results retrieved successfully", map[string]interface{}{
		"results":     results,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	}, "")
}

// ek specific scan ke results
func GetScanResult(c *fiber.Ctx) error {
	start := time.Now()
	requestID := uuid.New().String()

	scanID := c.Params("scan_id")
	log.Printf(
		"op=GetScanResult stage=start request_id=%s method=%s path=%s scan_id=%s ip=%s user_agent=%q",
		requestID, c.Method(), c.OriginalURL(), scanID, c.IP(), c.Get("User-Agent"),
	)

	if scanID == "" {
		log.Printf(
			"op=GetScanResult stage=validation_error request_id=%s error=%q elapsed=%s",
			requestID, "missing scan_id", time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Scan ID is required", nil, "")
	}

	objectID, err := primitive.ObjectIDFromHex(scanID)
	if err != nil {
		log.Printf(
			"op=GetScanResult stage=validation_error request_id=%s scan_id=%s error=%q elapsed=%s",
			requestID, scanID, "invalid hex", time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Invalid scan ID format", nil, "")
	}

	scanResult := &models.SCAN_RESULT{}
	err = mgm.Coll(scanResult).FindByID(objectID, scanResult)
	if err != nil {
		log.Printf(
			"op=GetScanResult stage=not_found request_id=%s scan_id=%s elapsed=%s",
			requestID, scanID, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusNotFound, "Scan result not found", nil, "")
	}

	totalFindings := 0
	findingsByType := make(map[string]int)
	findingsByResource := make(map[string]int)

	for _, resource := range scanResult.ScannedResources {

		findingCount := len(resource.Findings)
		totalFindings += findingCount
		findingsByResource[resource.Type+"_"+resource.ID] = findingCount
		for _, finding := range resource.Findings {
			findingsByType[finding.SecretType]++
		}
	}

	log.Printf(
		"op=GetScanResult stage=success request_id=%s scan_id=%s resources=%d total_findings=%d elapsed=%s",
		requestID, scanResult.ID.Hex(), len(scanResult.ScannedResources), totalFindings, time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Scan result retrieved successfully", map[string]interface{}{
		"scan_id":              scanResult.ID.Hex(),
		"request_id":           scanResult.RequestID,
		"scanned_resources":    scanResult.ScannedResources,
		"total_findings":       totalFindings,
		"findings_by_type":     findingsByType,
		"findings_by_resource": findingsByResource,
		"created_at":           scanResult.CreatedAt,
		"updated_at":           scanResult.UpdatedAt,
	}, "")
}
