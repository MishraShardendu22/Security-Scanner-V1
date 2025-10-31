package controller

import (
	"fmt"
	"log"
	"strings"
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

func ScanRequest(c *fiber.Ctx) error {
	start := time.Now()
	requestID := uuid.New().String()

	reqID := c.Params("request_id")
	log.Printf(
		"op=ScanRequest stage=start request_id=%s method=%s path=%s req_id=%s ip=%s user_agent=%q",
		requestID, c.Method(), c.OriginalURL(), reqID, c.IP(), c.Get("User-Agent"),
	)

	if reqID == "" {
		log.Printf(
			"op=ScanRequest stage=validation_error request_id=%s error=%q elapsed=%s",
			requestID, "missing request_id", time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Request ID is required", nil, "")
	}

	aiRequest := &models.AI_REQUEST{}
	err := mgm.Coll(aiRequest).First(bson.M{"request_id": reqID}, aiRequest)
	if err != nil {
		log.Printf(
			"op=ScanRequest stage=request_not_found request_id=%s req_id=%s elapsed=%s",
			requestID, reqID, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusNotFound, "Request not found", nil, "")
	}

	log.Printf(
		"op=ScanRequest stage=request_loaded request_id=%s req_id=%s files=%d discussions=%d",
		requestID, reqID, len(aiRequest.Siblings), len(aiRequest.Discussions),
	)

	resourceType := aiRequest.ResourceType
	resourceID := aiRequest.ResourceID
	if resourceType == "" {
		resourceType = "unknown"
	}
	if resourceID == "" {
		resourceID = "unknown"
	}

	log.Printf(
		"op=ScanRequest stage=scanning_start request_id=%s req_id=%s resource_type=%s resource_id=%s",
		requestID, reqID, resourceType, resourceID,
	)

	findings := util.ScanAIRequest(*aiRequest, util.SecretConfig, resourceType, resourceID)

	log.Printf(
		"op=ScanRequest stage=scanning_done request_id=%s req_id=%s findings=%d elapsed=%s",
		requestID, reqID, len(findings), time.Since(start),
	)

	scannedResources := util.GroupFindingsByResource(findings)

	scanResult := &models.SCAN_RESULT{
		RequestID:        reqID,
		ScannedResources: scannedResources,
	}

	if err := mgm.Coll(scanResult).Create(scanResult); err != nil {
		log.Printf(
			"op=ScanRequest stage=db_create_error request_id=%s req_id=%s error=%v elapsed=%s",
			requestID, reqID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to save scan results", nil, "")
	}

	findingsByType := util.CountFindingsByType(findings)
	findingsBySource := util.CountFindingsBySource(findings)

	log.Printf(
		"op=ScanRequest stage=success request_id=%s req_id=%s scan_id=%s total_findings=%d resources=%d total_elapsed=%s",
		requestID, reqID, scanResult.ID.Hex(), len(findings), len(scannedResources), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Scan completed successfully", map[string]interface{}{
		"scan_id":            scanResult.ID.Hex(),
		"request_id":         reqID,
		"total_findings":     len(findings),
		"findings_by_type":   findingsByType,
		"findings_by_source": findingsBySource,
		"scanned_resources":  scannedResources,
	}, "")
}

func ScanOrgModels(c *fiber.Ctx) error {
	start := time.Now()
	requestID := uuid.New().String()

	org := c.Params("org")
	log.Printf(
		"op=ScanOrgModels stage=start request_id=%s method=%s path=%s org=%s ip=%s user_agent=%q",
		requestID, c.Method(), c.OriginalURL(), org, c.IP(), c.Get("User-Agent"),
	)

	if org == "" {
		log.Printf(
			"op=ScanOrgModels stage=validation_error request_id=%s error=%q elapsed=%s",
			requestID, "missing org", time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Organization name is required", nil, "")
	}

	allFindings, scannedCount, err := util.ScanOrgResources(org, util.ResourceTypeModel)
	if err != nil {
		notFoundMsg := fmt.Sprintf("no %s found for this organization", util.ResourceTypeModel)
		if strings.Contains(err.Error(), notFoundMsg) {
			includePRs, includeDiscussion := util.ParseIncludeFlags(c)
			log.Printf(
				"op=ScanOrgModels stage=auto_populate_start request_id=%s org=%s include_prs=%t include_discussion=%t",
				requestID, org, includePRs, includeDiscussion,
			)
			_, saved, fetchErr := util.FetchOrgResources(c, org, util.ResourceTypeModel, includePRs, includeDiscussion)
			if fetchErr != nil {
				log.Printf(
					"op=ScanOrgModels stage=auto_populate_error request_id=%s org=%s error=%v elapsed=%s",
					requestID, org, fetchErr, time.Since(start),
				)
				log.Printf(
					"op=ScanOrgModels stage=not_found_fallback_live_scan request_id=%s org=%s elapsed=%s",
					requestID, org, time.Since(start),
				)
				return scanOrganization(c, org, includePRs, includeDiscussion, "org-scan-"+org)
			}

			log.Printf(
				"op=ScanOrgModels stage=auto_populate_done request_id=%s org=%s saved=%d elapsed=%s",
				requestID, org, len(saved), time.Since(start),
			)

			if len(saved) > 0 {
				log.Printf(
					"op=ScanOrgModels stage=scan_by_ids_start request_id=%s org=%s ids=%d",
					requestID, org, len(saved),
				)
				allFindings, scannedCount, err = util.ScanResourcesByIDs(saved, util.ResourceTypeModel, includePRs, includeDiscussion)
				if err != nil {
					log.Printf(
						"op=ScanOrgModels stage=scan_by_ids_error request_id=%s org=%s error=%v elapsed=%s",
						requestID, org, err, time.Since(start),
					)
					return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
				}
			} else {
				log.Printf(
					"op=ScanOrgModels stage=auto_populate_none request_id=%s org=%s elapsed=%s",
					requestID, org, time.Since(start),
				)
				return scanOrganization(c, org, includePRs, includeDiscussion, "org-scan-"+org)
			}
		} else {
			log.Printf(
				"op=ScanOrgModels stage=scan_error request_id=%s org=%s error=%v elapsed=%s",
				requestID, org, err, time.Since(start),
			)
			return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
		}
	}

	scannedResources := util.GroupFindingsByResource(allFindings)
	scanResult, err := util.SaveScanResults("org-scan-"+org+"-models", scannedResources)
	if err != nil {
		log.Printf(
			"op=ScanOrgModels stage=db_create_error request_id=%s org=%s error=%v elapsed=%s",
			requestID, org, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
	}

	log.Printf(
		"op=ScanOrgModels stage=success request_id=%s org=%s scan_id=%s models_scanned=%d findings=%d resources=%d elapsed=%s",
		requestID, org, scanResult.ID.Hex(), scannedCount, len(allFindings), len(scannedResources), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Organization models scanned successfully", map[string]interface{}{
		"scan_id":           scanResult.ID.Hex(),
		"organization":      org,
		"models_scanned":    scannedCount,
		"total_findings":    len(allFindings),
		"scanned_resources": scannedResources,
	}, "")
}

func ScanOrgDatasets(c *fiber.Ctx) error {
	start := time.Now()
	requestID := uuid.New().String()

	org := c.Params("org")
	log.Printf(
		"op=ScanOrgDatasets stage=start request_id=%s method=%s path=%s org=%s ip=%s user_agent=%q",
		requestID, c.Method(), c.OriginalURL(), org, c.IP(), c.Get("User-Agent"),
	)

	if org == "" {
		log.Printf(
			"op=ScanOrgDatasets stage=validation_error request_id=%s error=%q elapsed=%s",
			requestID, "missing org", time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Organization name is required", nil, "")
	}

	allFindings, scannedCount, err := util.ScanOrgResources(org, util.ResourceTypeDataset)
	if err != nil {
		notFoundMsg := fmt.Sprintf("no %s found for this organization", util.ResourceTypeDataset)
		if strings.Contains(err.Error(), notFoundMsg) {
			includePRs, includeDiscussion := util.ParseIncludeFlags(c)
			log.Printf(
				"op=ScanOrgDatasets stage=auto_populate_start request_id=%s org=%s include_prs=%t include_discussion=%t",
				requestID, org, includePRs, includeDiscussion,
			)
			_, saved, fetchErr := util.FetchOrgResources(c, org, util.ResourceTypeDataset, includePRs, includeDiscussion)
			if fetchErr != nil {
				log.Printf(
					"op=ScanOrgDatasets stage=auto_populate_error request_id=%s org=%s error=%v elapsed=%s",
					requestID, org, fetchErr, time.Since(start),
				)
				log.Printf(
					"op=ScanOrgDatasets stage=not_found_fallback_live_scan request_id=%s org=%s elapsed=%s",
					requestID, org, time.Since(start),
				)
				return scanOrganization(c, org, includePRs, includeDiscussion, "org-scan-"+org)
			}
			log.Printf(
				"op=ScanOrgDatasets stage=auto_populate_done request_id=%s org=%s saved=%d elapsed=%s",
				requestID, org, len(saved), time.Since(start),
			)
			if len(saved) > 0 {
				log.Printf(
					"op=ScanOrgDatasets stage=scan_by_ids_start request_id=%s org=%s ids=%d",
					requestID, org, len(saved),
				)
				allFindings, scannedCount, err = util.ScanResourcesByIDs(saved, util.ResourceTypeDataset, includePRs, includeDiscussion)
				if err != nil {
					log.Printf(
						"op=ScanOrgDatasets stage=scan_by_ids_error request_id=%s org=%s error=%v elapsed=%s",
						requestID, org, err, time.Since(start),
					)
					return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
				}
			} else {
				log.Printf(
					"op=ScanOrgDatasets stage=auto_populate_none request_id=%s org=%s elapsed=%s",
					requestID, org, time.Since(start),
				)
				return scanOrganization(c, org, includePRs, includeDiscussion, "org-scan-"+org)
			}
		} else {
			log.Printf(
				"op=ScanOrgDatasets stage=scan_error request_id=%s org=%s error=%v elapsed=%s",
				requestID, org, err, time.Since(start),
			)
			return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
		}
	}

	scannedResources := util.GroupFindingsByResource(allFindings)
	scanResult, err := util.SaveScanResults("org-scan-"+org+"-datasets", scannedResources)
	if err != nil {
		log.Printf(
			"op=ScanOrgDatasets stage=db_create_error request_id=%s org=%s error=%v elapsed=%s",
			requestID, org, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
	}

	log.Printf(
		"op=ScanOrgDatasets stage=success request_id=%s org=%s scan_id=%s datasets_scanned=%d findings=%d resources=%d elapsed=%s",
		requestID, org, scanResult.ID.Hex(), scannedCount, len(allFindings), len(scannedResources), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Organization datasets scanned successfully", map[string]interface{}{
		"scan_id":           scanResult.ID.Hex(),
		"organization":      org,
		"datasets_scanned":  scannedCount,
		"total_findings":    len(allFindings),
		"scanned_resources": scannedResources,
	}, "")
}

func ScanOrgSpaces(c *fiber.Ctx) error {
	start := time.Now()
	requestID := uuid.New().String()

	org := c.Params("org")
	log.Printf(
		"op=ScanOrgSpaces stage=start request_id=%s method=%s path=%s org=%s ip=%s user_agent=%q",
		requestID, c.Method(), c.OriginalURL(), org, c.IP(), c.Get("User-Agent"),
	)

	if org == "" {
		log.Printf(
			"op=ScanOrgSpaces stage=validation_error request_id=%s error=%q elapsed=%s",
			requestID, "missing org", time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Organization name is required", nil, "")
	}

	allFindings, scannedCount, err := util.ScanOrgResources(org, util.ResourceTypeSpace)
	if err != nil {
		notFoundMsg := fmt.Sprintf("no %s found for this organization", util.ResourceTypeSpace)
		if strings.Contains(err.Error(), notFoundMsg) {
			includePRs, includeDiscussion := util.ParseIncludeFlags(c)
			log.Printf(
				"op=ScanOrgSpaces stage=auto_populate_start request_id=%s org=%s include_prs=%t include_discussion=%t",
				requestID, org, includePRs, includeDiscussion,
			)
			_, saved, fetchErr := util.FetchOrgResources(c, org, util.ResourceTypeSpace, includePRs, includeDiscussion)
			if fetchErr != nil {
				log.Printf(
					"op=ScanOrgSpaces stage=auto_populate_error request_id=%s org=%s error=%v elapsed=%s",
					requestID, org, fetchErr, time.Since(start),
				)
				log.Printf(
					"op=ScanOrgSpaces stage=not_found_fallback_live_scan request_id=%s org=%s elapsed=%s",
					requestID, org, time.Since(start),
				)
				return scanOrganization(c, org, includePRs, includeDiscussion, "org-scan-"+org)
			}
			log.Printf(
				"op=ScanOrgSpaces stage=auto_populate_done request_id=%s org=%s saved=%d elapsed=%s",
				requestID, org, len(saved), time.Since(start),
			)
			if len(saved) > 0 {
				log.Printf(
					"op=ScanOrgSpaces stage=scan_by_ids_start request_id=%s org=%s ids=%d",
					requestID, org, len(saved),
				)
				allFindings, scannedCount, err = util.ScanResourcesByIDs(saved, util.ResourceTypeSpace, includePRs, includeDiscussion)
				if err != nil {
					log.Printf(
						"op=ScanOrgSpaces stage=scan_by_ids_error request_id=%s org=%s error=%v elapsed=%s",
						requestID, org, err, time.Since(start),
					)
					return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
				}
			} else {
				log.Printf(
					"op=ScanOrgSpaces stage=auto_populate_none request_id=%s org=%s elapsed=%s",
					requestID, org, time.Since(start),
				)
				return scanOrganization(c, org, includePRs, includeDiscussion, "org-scan-"+org)
			}
		} else {
			log.Printf(
				"op=ScanOrgSpaces stage=scan_error request_id=%s org=%s error=%v elapsed=%s",
				requestID, org, err, time.Since(start),
			)
			return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
		}
	}

	scannedResources := util.GroupFindingsByResource(allFindings)
	scanResult, err := util.SaveScanResults("org-scan-"+org+"-spaces", scannedResources)
	if err != nil {
		log.Printf(
			"op=ScanOrgSpaces stage=db_create_error request_id=%s org=%s error=%v elapsed=%s",
			requestID, org, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
	}

	log.Printf(
		"op=ScanOrgSpaces stage=success request_id=%s org=%s scan_id=%s spaces_scanned=%d findings=%d resources=%d elapsed=%s",
		requestID, org, scanResult.ID.Hex(), scannedCount, len(allFindings), len(scannedResources), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Organization spaces scanned successfully", map[string]interface{}{
		"scan_id":           scanResult.ID.Hex(),
		"organization":      org,
		"spaces_scanned":    scannedCount,
		"total_findings":    len(allFindings),
		"scanned_resources": scannedResources,
	}, "")
}

func ScanByID(c *fiber.Ctx) error {
	start := time.Now()
	requestID := uuid.New().String()

	id := c.Params("id")
	log.Printf(
		"op=ScanByID stage=start request_id=%s method=%s path=%s id=%s ip=%s user_agent=%q",
		requestID, c.Method(), c.OriginalURL(), id, c.IP(), c.Get("User-Agent"),
	)

	if id == "" {
		log.Printf(
			"op=ScanByID stage=validation_error request_id=%s error=%q elapsed=%s",
			requestID, "missing id", time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusBadRequest, "ID is required", nil, "")
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Printf(
			"op=ScanByID stage=validation_error request_id=%s id=%s error=%q elapsed=%s",
			requestID, id, "invalid hex", time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Invalid ID format", nil, "")
	}

	aiRequest := &models.AI_REQUEST{}
	err = mgm.Coll(aiRequest).FindByID(objectID, aiRequest)
	if err != nil {
		log.Printf(
			"op=ScanByID stage=request_not_found request_id=%s id=%s elapsed=%s",
			requestID, id, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusNotFound, "Request not found", nil, "")
	}

	resourceType := aiRequest.ResourceType
	resourceID := aiRequest.ResourceID
	if resourceType == "" {
		resourceType = "unknown"
	}
	if resourceID == "" {
		resourceID = "unknown"
	}

	log.Printf(
		"op=ScanByID stage=scanning_start request_id=%s id=%s resource_type=%s resource_id=%s",
		requestID, id, resourceType, resourceID,
	)

	findings := util.ScanAIRequest(*aiRequest, util.SecretConfig, resourceType, resourceID)
	scannedResources := util.GroupFindingsByResource(findings)

	scanResult := &models.SCAN_RESULT{
		RequestID:        aiRequest.RequestID,
		ScannedResources: scannedResources,
	}
	if err := mgm.Coll(scanResult).Create(scanResult); err != nil {
		log.Printf(
			"op=ScanByID stage=db_create_error request_id=%s id=%s error=%v elapsed=%s",
			requestID, id, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to save scan results", nil, "")
	}

	log.Printf(
		"op=ScanByID stage=success request_id=%s id=%s scan_id=%s total_findings=%d resources=%d elapsed=%s",
		requestID, id, scanResult.ID.Hex(), len(findings), len(scannedResources), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Scan completed successfully", map[string]interface{}{
		"scan_id":           scanResult.ID.Hex(),
		"request_id":        aiRequest.RequestID,
		"total_findings":    len(findings),
		"scanned_resources": scannedResources,
	}, "")
}
