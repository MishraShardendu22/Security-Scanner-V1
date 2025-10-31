package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/MishraShardendu22/Scanner/models"
	"github.com/MishraShardendu22/Scanner/util"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kamva/mgm/v3"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

var httpClient = util.SharedHTTPClient()

func UnifiedScan(c *fiber.Ctx) error {
	start := time.Now()
	traceID := uuid.New().String()

	var req models.ScanRequestBody
	if err := c.BodyParser(&req); err != nil {
		log.Printf(
			"op=UnifiedScan stage=body_parse_error trace_id=%s method=%s path=%s error=%v elapsed=%s",
			traceID, c.Method(), c.OriginalURL(), err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Invalid request body", nil, "")
	}

	log.Printf(
		"op=UnifiedScan stage=start trace_id=%s method=%s path=%s ip=%s user_agent=%q model_id=%s dataset_id=%s space_id=%s org=%s user=%s include_prs=%t include_discussions=%t",
		traceID, c.Method(), c.OriginalURL(), c.IP(), c.Get("User-Agent"),
		req.ModelID, req.DatasetID, req.SpaceID, req.Org, req.User, req.IncludePRs, req.IncludeDiscussions,
	)

	scanID := fmt.Sprintf("SG-%s-%s", time.Now().Format("2006-0102"), uuid.New().String()[:8])
	requestID := uuid.New().String()

	aiRequest := &models.AI_REQUEST{
		RequestID:   requestID,
		Siblings:    []models.SIBLING{},
		Discussions: []models.DISCUSSION{},
	}

	var scannedResources []models.SCANNED_RESOURCE
	var resourceType, resourceID string

	if req.ModelID != "" {
		resourceType = "models"
		resourceID = req.ModelID
		aiRequest.ResourceType = resourceType
		aiRequest.ResourceID = resourceID

		log.Printf(
			"op=UnifiedScan stage=fetch_model_start trace_id=%s request_id=%s model_id=%s",
			traceID, requestID, req.ModelID,
		)
		if err := fetchAndAddToRequest(aiRequest, req.ModelID, "models", req.IncludePRs, req.IncludeDiscussions); err != nil {
			log.Printf(
				"op=UnifiedScan stage=fetch_model_error trace_id=%s request_id=%s model_id=%s error=%v elapsed=%s",
				traceID, requestID, req.ModelID, err, time.Since(start),
			)
			return util.ResponseAPI(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to fetch model: %v", err), nil, "")
		}
		log.Printf(
			"op=UnifiedScan stage=fetch_model_done trace_id=%s request_id=%s model_id=%s files=%d discussions=%d",
			traceID, requestID, req.ModelID, len(aiRequest.Siblings), len(aiRequest.Discussions),
		)
	} else if req.DatasetID != "" {
		resourceType = "datasets"
		resourceID = req.DatasetID
		aiRequest.ResourceType = resourceType
		aiRequest.ResourceID = resourceID

		log.Printf(
			"op=UnifiedScan stage=fetch_dataset_start trace_id=%s request_id=%s dataset_id=%s",
			traceID, requestID, req.DatasetID,
		)
		if err := fetchAndAddToRequest(aiRequest, req.DatasetID, "datasets", req.IncludePRs, req.IncludeDiscussions); err != nil {
			log.Printf(
				"op=UnifiedScan stage=fetch_dataset_error trace_id=%s request_id=%s dataset_id=%s error=%v elapsed=%s",
				traceID, requestID, req.DatasetID, err, time.Since(start),
			)
			return util.ResponseAPI(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to fetch dataset: %v", err), nil, "")
		}
		log.Printf(
			"op=UnifiedScan stage=fetch_dataset_done trace_id=%s request_id=%s dataset_id=%s files=%d discussions=%d",
			traceID, requestID, req.DatasetID, len(aiRequest.Siblings), len(aiRequest.Discussions),
		)
	} else if req.SpaceID != "" {
		resourceType = "spaces"
		resourceID = req.SpaceID
		aiRequest.ResourceType = resourceType
		aiRequest.ResourceID = resourceID

		log.Printf(
			"op=UnifiedScan stage=fetch_space_start trace_id=%s request_id=%s space_id=%s",
			traceID, requestID, req.SpaceID,
		)
		if err := fetchAndAddToRequest(aiRequest, req.SpaceID, "spaces", req.IncludePRs, req.IncludeDiscussions); err != nil {
			log.Printf(
				"op=UnifiedScan stage=fetch_space_error trace_id=%s request_id=%s space_id=%s error=%v elapsed=%s",
				traceID, requestID, req.SpaceID, err, time.Since(start),
			)
			return util.ResponseAPI(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to fetch space: %v", err), nil, "")
		}
		log.Printf(
			"op=UnifiedScan stage=fetch_space_done trace_id=%s request_id=%s space_id=%s files=%d discussions=%d",
			traceID, requestID, req.SpaceID, len(aiRequest.Siblings), len(aiRequest.Discussions),
		)
	} else if req.Org != "" {
		log.Printf(
			"op=UnifiedScan stage=org_scan_start trace_id=%s org=%s include_prs=%t include_discussions=%t",
			traceID, req.Org, req.IncludePRs, req.IncludeDiscussions,
		)
		return scanOrganization(c, req.Org, req.IncludePRs, req.IncludeDiscussions, scanID)
	} else if req.User != "" {
		log.Printf(
			"op=UnifiedScan stage=user_scan_start trace_id=%s user=%s include_prs=%t include_discussions=%t",
			traceID, req.User, req.IncludePRs, req.IncludeDiscussions,
		)
		return scanOrganization(c, req.User, req.IncludePRs, req.IncludeDiscussions, scanID)
	} else {
		log.Printf(
			"op=UnifiedScan stage=validation_error trace_id=%s error=%q elapsed=%s",
			traceID, "missing one of model_id,dataset_id,space_id,org,user", time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusBadRequest, "At least one of model_id, dataset_id, space_id, org, or user is required", nil, "")
	}

	log.Printf(
		"op=UnifiedScan stage=scanning_start trace_id=%s request_id=%s resource_type=%s resource_id=%s files=%d discussions=%d",
		traceID, requestID, resourceType, resourceID, len(aiRequest.Siblings), len(aiRequest.Discussions),
	)
	findings := util.ScanAIRequest(*aiRequest, util.SecretConfig, resourceType, resourceID)
	log.Printf(
		"op=UnifiedScan stage=scanning_done trace_id=%s request_id=%s findings=%d elapsed=%s",
		traceID, requestID, len(findings), time.Since(start),
	)

	findingsMap := make(map[string][]models.Finding)
	for _, finding := range findings {
		var key string
		if finding.SourceType == "file" {
			key = "file:" + finding.FileName
		} else {
			key = "discussion:" + finding.DiscussionTitle
		}
		findingsMap[key] = append(findingsMap[key], finding)
	}

	resourceFindings := []models.Finding{}
	for _, findingsList := range findingsMap {
		resourceFindings = append(resourceFindings, findingsList...)
	}

	scannedResource := models.SCANNED_RESOURCE{
		Type:     resourceType,
		ID:       resourceID,
		Findings: resourceFindings,
	}
	scannedResources = append(scannedResources, scannedResource)

	log.Printf(
		"op=UnifiedScan stage=save_scan_result_start trace_id=%s request_id=%s scanned_resources=%d total_findings=%d",
		traceID, requestID, len(scannedResources), len(findings),
	)
	scanResult := &models.SCAN_RESULT{
		RequestID:        requestID,
		ScannedResources: scannedResources,
	}
	if err := mgm.Coll(scanResult).Create(scanResult); err != nil {
		log.Printf(
			"op=UnifiedScan stage=save_scan_result_error trace_id=%s request_id=%s error=%v elapsed=%s",
			traceID, requestID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to save scan results", nil, "")
	}
	log.Printf(
		"op=UnifiedScan stage=save_scan_result_done trace_id=%s request_id=%s scan_id=%s storage_id=%s",
		traceID, requestID, scanID, scanResult.ID.Hex(),
	)

	response := map[string]interface{}{
		"scan_id": scanID,
		"scanned_resources": []map[string]interface{}{
			{
				"type":     resourceType,
				"id":       resourceID,
				"findings": util.FormatFindings(resourceFindings),
			},
		},
		"timestamp":      time.Now().Format(time.RFC3339),
		"total_findings": len(findings),
		"storage_id":     scanResult.ID.Hex(),
	}

	log.Printf(
		"op=UnifiedScan stage=success trace_id=%s request_id=%s scan_id=%s total_findings=%d total_elapsed=%s",
		traceID, requestID, scanID, len(findings), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Scan completed successfully", response, "")
}

func StoreScanResult(c *fiber.Ctx) error {
	start := time.Now()
	traceID := uuid.New().String()

	var scanData map[string]interface{}
	if err := c.BodyParser(&scanData); err != nil {
		log.Printf(
			"op=StoreScanResult stage=body_parse_error trace_id=%s method=%s path=%s error=%v elapsed=%s",
			traceID, c.Method(), c.OriginalURL(), err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Invalid request body", nil, "")
	}

	scanIDValue, ok := scanData["scan_id"].(string)
	if !ok {
		scanIDValue = fmt.Sprintf("SG-%s-%s", time.Now().Format("2006-0102"), uuid.New().String()[:8])
	}

	log.Printf(
		"op=StoreScanResult stage=save_start trace_id=%s scan_id=%s",
		traceID, scanIDValue,
	)

	scanResult := &models.SCAN_RESULT{
		RequestID:        scanIDValue,
		ScannedResources: []models.SCANNED_RESOURCE{},
	}
	if resources, ok := scanData["scanned_resources"].([]interface{}); ok {
		for _, res := range resources {
			if resMap, ok := res.(map[string]interface{}); ok {
				resource := models.SCANNED_RESOURCE{}
				if resType, ok := resMap["type"].(string); ok {
					resource.Type = resType
				}
				if resID, ok := resMap["id"].(string); ok {
					resource.ID = resID
				}
				if findings, ok := resMap["findings"].([]interface{}); ok {
					for _, f := range findings {
						if fMap, ok := f.(map[string]interface{}); ok {
							finding := models.Finding{}
							if secretType, ok := fMap["secret_type"].(string); ok {
								finding.SecretType = secretType
							}
							if pattern, ok := fMap["pattern"].(string); ok {
								finding.Pattern = pattern
							}
							if secret, ok := fMap["secret"].(string); ok {
								finding.Secret = secret
							}
							if file, ok := fMap["file"].(string); ok {
								finding.FileName = file
								finding.SourceType = "file"
							}
							if line, ok := fMap["line"].(float64); ok {
								finding.Line = int(line)
							}
							resource.Findings = append(resource.Findings, finding)
						}
					}
				}
				scanResult.ScannedResources = append(scanResult.ScannedResources, resource)
			}
		}
	}
	if err := mgm.Coll(scanResult).Create(scanResult); err != nil {
		log.Printf(
			"op=StoreScanResult stage=save_error trace_id=%s scan_id=%s error=%v elapsed=%s",
			traceID, scanIDValue, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to store scan results", nil, "")
	}

	log.Printf(
		"op=StoreScanResult stage=success trace_id=%s scan_id=%s storage_id=%s elapsed=%s",
		traceID, scanIDValue, scanResult.ID.Hex(), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Scan results stored successfully", map[string]interface{}{
		"status":     "stored",
		"scan_id":    scanIDValue,
		"storage_id": scanResult.ID.Hex(),
	}, "")
}

func fetchAndAddToRequest(aiRequest *models.AI_REQUEST, resourceID, resourceType string, includePRs, includeDiscussions bool) error {
	start := time.Now()
	traceID := uuid.New().String()

	url := fmt.Sprintf("https://huggingface.co/api/%s/%s", resourceType, resourceID)
	log.Printf(
		"op=fetchAndAddToRequest stage=start trace_id=%s resource_type=%s resource_id=%s url=%s include_prs=%t include_discussions=%t",
		traceID, resourceType, resourceID, url, includePRs, includeDiscussions,
	)

	resp, err := httpClient.Get(url)
	if err != nil {
		log.Printf(
			"op=fetchAndAddToRequest stage=http_get_error trace_id=%s resource_type=%s resource_id=%s url=%s error=%v elapsed=%s",
			traceID, resourceType, resourceID, url, err, time.Since(start),
		)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf(
			"op=fetchAndAddToRequest stage=not_ok trace_id=%s resource_type=%s resource_id=%s status=%d elapsed=%s",
			traceID, resourceType, resourceID, resp.StatusCode, time.Since(start),
		)
		return fmt.Errorf("resource not found: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf(
			"op=fetchAndAddToRequest stage=read_error trace_id=%s resource_type=%s resource_id=%s error=%v elapsed=%s",
			traceID, resourceType, resourceID, err, time.Since(start),
		)
		return err
	}

	var resourceData map[string]interface{}
	if err := json.Unmarshal(body, &resourceData); err != nil {
		log.Printf(
			"op=fetchAndAddToRequest stage=json_unmarshal_error trace_id=%s resource_type=%s resource_id=%s error=%v elapsed=%s",
			traceID, resourceType, resourceID, err, time.Since(start),
		)
		return err
	}

	if siblings, ok := resourceData["siblings"].([]interface{}); ok {
		log.Printf(
			"op=fetchAndAddToRequest stage=fetch_files_start trace_id=%s resource_type=%s resource_id=%s sibling_candidates=%d",
			traceID, resourceType, resourceID, len(siblings),
		)
		aiRequest.Siblings = util.FetchFilesFromSiblings(resourceID, siblings)
		log.Printf(
			"op=fetchAndAddToRequest stage=fetch_files_done trace_id=%s resource_type=%s resource_id=%s files=%d",
			traceID, resourceType, resourceID, len(aiRequest.Siblings),
		)
	}

	if includePRs || includeDiscussions {
		log.Printf(
			"op=fetchAndAddToRequest stage=fetch_discussions_start trace_id=%s resource_type=%s resource_id=%s include_prs=%t include_discussions=%t",
			traceID, resourceType, resourceID, includePRs, includeDiscussions,
		)
		discussions, _ := util.FetchDiscussions(resourceID, resourceType, includePRs, includeDiscussions)
		aiRequest.Discussions = discussions
		log.Printf(
			"op=fetchAndAddToRequest stage=fetch_discussions_done trace_id=%s resource_type=%s resource_id=%s discussions=%d",
			traceID, resourceType, resourceID, len(discussions),
		)
	}

	log.Printf(
		"op=fetchAndAddToRequest stage=success trace_id=%s resource_type=%s resource_id=%s elapsed=%s",
		traceID, resourceType, resourceID, time.Since(start),
	)

	return nil
}

func scanOrganization(c *fiber.Ctx, org string, includePRs, includeDiscussions bool, scanID string) error {
	start := time.Now()
	traceID := uuid.New().String()

	modelsURL := fmt.Sprintf("https://huggingface.co/api/models?author=%s&full=true", org)
	log.Printf(
		"op=scanOrganization stage=start trace_id=%s org=%s include_prs=%t include_discussions=%t url=%s",
		traceID, org, includePRs, includeDiscussions, modelsURL,
	)

	resp, err := httpClient.Get(modelsURL)
	if err != nil {
		log.Printf(
			"op=scanOrganization stage=http_get_error trace_id=%s org=%s url=%s error=%v elapsed=%s",
			traceID, org, modelsURL, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to fetch organization models", nil, "")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf(
			"op=scanOrganization stage=read_error trace_id=%s org=%s error=%v elapsed=%s",
			traceID, org, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to read response", nil, "")
	}

	var modelsData []map[string]interface{}
	if err := json.Unmarshal(body, &modelsData); err != nil {
		log.Printf(
			"op=scanOrganization stage=json_unmarshal_error trace_id=%s org=%s error=%v elapsed=%s",
			traceID, org, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to parse response", nil, "")
	}

	log.Printf(
		"op=scanOrganization stage=parsed trace_id=%s org=%s models=%d",
		traceID, org, len(modelsData),
	)

	var allScannedResources []models.SCANNED_RESOURCE
	var totalFindings int
	var mu sync.Mutex

	limit := 10
	if len(modelsData) < limit {
		limit = len(modelsData)
	}

	log.Printf(
		"op=scanOrganization stage=scanning_models_start trace_id=%s org=%s concurrent=%d limit=%d",
		traceID, org, 10, limit,
	)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)

	for i := 0; i < limit; i++ {
		modelData := modelsData[i]
		modelID, ok := modelData["id"].(string)
		if !ok {
			log.Printf(
				"op=scanOrganization stage=skip_missing_id trace_id=%s org=%s index=%d",
				traceID, org, i,
			)
			continue
		}
		wg.Add(1)
		go func(id string, index int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			localStart := time.Now()
			log.Printf(
				"op=scanOrganization stage=scan_model_start trace_id=%s org=%s index=%d total=%d model_id=%s",
				traceID, org, index+1, limit, id,
			)

			aiRequest := &models.AI_REQUEST{
				RequestID:    uuid.New().String(),
				ResourceType: "models",
				ResourceID:   id,
				Siblings:     []models.SIBLING{},
				Discussions:  []models.DISCUSSION{},
			}

			if err := fetchAndAddToRequest(aiRequest, id, "models", includePRs, includeDiscussions); err != nil {
				log.Printf(
					"op=scanOrganization stage=fetch_model_error trace_id=%s org=%s model_id=%s error=%v elapsed=%s",
					traceID, org, id, err, time.Since(localStart),
				)
				return
			}

			findings := util.ScanAIRequest(*aiRequest, util.SecretConfig, "models", id)

			mu.Lock()
			totalFindings += len(findings)
			mu.Unlock()

			if len(findings) > 0 {
				log.Printf(
					"op=scanOrganization stage=scan_model_findings trace_id=%s org=%s model_id=%s findings=%d elapsed=%s",
					traceID, org, id, len(findings), time.Since(localStart),
				)
				scannedResource := models.SCANNED_RESOURCE{
					Type:     "model",
					ID:       id,
					Findings: findings,
				}
				mu.Lock()
				allScannedResources = append(allScannedResources, scannedResource)
				mu.Unlock()
			} else {
				log.Printf(
					"op=scanOrganization stage=scan_model_no_findings trace_id=%s org=%s model_id=%s elapsed=%s",
					traceID, org, id, time.Since(localStart),
				)
			}
		}(modelID, i)
	}
	wg.Wait()

	log.Printf(
		"op=scanOrganization stage=scanning_models_done trace_id=%s org=%s total_findings=%d",
		traceID, org, totalFindings,
	)

	scanResult := &models.SCAN_RESULT{
		RequestID:        "org-" + org,
		ScannedResources: allScannedResources,
	}
	if err := mgm.Coll(scanResult).Create(scanResult); err != nil {
		log.Printf(
			"op=scanOrganization stage=db_create_error trace_id=%s org=%s error=%v elapsed=%s",
			traceID, org, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to save scan results", nil, "")
	}

	formattedResources := []map[string]interface{}{}
	for _, resource := range allScannedResources {
		formattedResources = append(formattedResources, map[string]interface{}{
			"type":     resource.Type,
			"id":       resource.ID,
			"findings": util.FormatFindings(resource.Findings),
		})
	}

	response := map[string]interface{}{
		"scan_id":           scanID,
		"scanned_resources": formattedResources,
		"timestamp":         time.Now().Format(time.RFC3339),
		"total_findings":    totalFindings,
		"models_scanned":    limit,
		"storage_id":        scanResult.ID.Hex(),
	}

	log.Printf(
		"op=scanOrganization stage=success trace_id=%s org=%s scan_id=%s total_findings=%d models_scanned=%d storage_id=%s elapsed=%s",
		traceID, org, scanID, totalFindings, limit, scanResult.ID.Hex(), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Organization scan completed successfully", response, "")
}

/*
mistake -
database me file nahi store kar rahe h 16 mb limit og mongodb, in memory save kar rahe h
instead we can save the url for the file and scan it in memory.

i was saving in database cause waht if server crashes mid scan atleast we have data in DATABASE
but wrong appraoch database me file save karna
file are always available , even on server restart we will have to start agian cant resume rigth ?
*/
