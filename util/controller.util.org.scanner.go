package util

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/MishraShardendu22/Scanner/models"
	"github.com/google/uuid"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

// simple scan database me save karta h scan
func ScanOrgResources(
	org string,
	resourceType ResourceType,
) ([]models.Finding, int, error) {
	start := time.Now()
	traceID := uuid.New().String()

	log.Printf(
		"op=ScanOrgResources stage=start trace_id=%s org=%s resource_type=%s",
		traceID, org, resourceType,
	)

	var count int64
	var err error

	switch resourceType {
	case ResourceTypeModel:
		count, err = mgm.Coll(&models.AI_Models{}).CountDocuments(mgm.Ctx(), bson.M{"org": org})
	case ResourceTypeDataset:
		count, err = mgm.Coll(&models.AI_DATASETS{}).CountDocuments(mgm.Ctx(), bson.M{"org": org})
	case ResourceTypeSpace:
		count, err = mgm.Coll(&models.AI_SPACES{}).CountDocuments(mgm.Ctx(), bson.M{"org": org})
	default:
		log.Printf(
			"op=ScanOrgResources stage=validation_error trace_id=%s org=%s resource_type=%s error=%q elapsed=%s",
			traceID, org, resourceType, "unsupported resource type", time.Since(start),
		)
		return nil, 0, fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	if err != nil {
		log.Printf(
			"op=ScanOrgResources stage=count_error trace_id=%s org=%s resource_type=%s error=%v elapsed=%s",
			traceID, org, resourceType, err, time.Since(start),
		)
		return nil, 0, fmt.Errorf("failed to fetch organization %s", resourceType)
	}
	if count == 0 {
		log.Printf(
			"op=ScanOrgResources stage=not_found trace_id=%s org=%s resource_type=%s elapsed=%s",
			traceID, org, resourceType, time.Since(start),
		)
		return nil, 0, fmt.Errorf("no %s found for this organization", resourceType)
	}

	log.Printf(
		"op=ScanOrgResources stage=found trace_id=%s org=%s resource_type=%s count=%d",
		traceID, org, resourceType, count,
	)

	var allFindings []models.Finding
	var scannedCount int

	aiRequests := []models.AI_REQUEST{}
	if err := mgm.Coll(&models.AI_REQUEST{}).SimpleFind(&aiRequests, bson.M{}); err != nil {
		log.Printf(
			"op=ScanOrgResources stage=query_requests_error trace_id=%s org=%s resource_type=%s error=%v elapsed=%s",
			traceID, org, resourceType, err, time.Since(start),
		)
		return nil, 0, fmt.Errorf("failed to load requests")
	}

	concurrency := 10
	log.Printf(
		"op=ScanOrgResources stage=scanning_start trace_id=%s org=%s resource_type=%s requests=%d concurrency=%d",
		traceID, org, resourceType, len(aiRequests), concurrency,
	)

	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, concurrency)

	for idx, req := range aiRequests {
		wg.Add(1)
		go func(r models.AI_REQUEST, index int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			localStart := time.Now()
			log.Printf(
				"op=ScanOrgResources stage=scan_request_start trace_id=%s org=%s resource_type=%s index=%d total=%d request_id=%s",
				traceID, org, resourceType, index+1, len(aiRequests), r.RequestID,
			)

			resType := r.ResourceType
			resID := r.ResourceID
			if resType == "" {
				resType = string(resourceType)
			}
			if resID == "" {
				resID = "unknown"
			}

			findings := ScanAIRequest(r, SecretConfig, resType, resID)

			mu.Lock()
			allFindings = append(allFindings, findings...)
			scannedCount++
			mu.Unlock()

			if len(findings) > 0 {
				log.Printf(
					"op=ScanOrgResources stage=scan_request_findings trace_id=%s request_id=%s findings=%d elapsed=%s",
					traceID, r.RequestID, len(findings), time.Since(localStart),
				)
			} else {
				log.Printf(
					"op=ScanOrgResources stage=scan_request_no_findings trace_id=%s request_id=%s elapsed=%s",
					traceID, r.RequestID, time.Since(localStart),
				)
			}
		}(req, idx)
	}
	wg.Wait()

	log.Printf(
		"op=ScanOrgResources stage=success trace_id=%s org=%s resource_type=%s total_findings=%d scanned_requests=%d total_elapsed=%s",
		traceID, org, resourceType, len(allFindings), scannedCount, time.Since(start),
	)

	return allFindings, scannedCount, nil
}

// results ko database me upar scan results save karne ke liye
func SaveScanResults(requestID string, scannedResources []models.SCANNED_RESOURCE) (*models.SCAN_RESULT, error) {
	start := time.Now()
	traceID := uuid.New().String()

	log.Printf(
		"op=SaveScanResults stage=start trace_id=%s request_id=%s resources=%d",
		traceID, requestID, len(scannedResources),
	)

	scanResult := &models.SCAN_RESULT{
		RequestID:        requestID,
		ScannedResources: scannedResources,
	}
	if err := mgm.Coll(scanResult).Create(scanResult); err != nil {
		log.Printf(
			"op=SaveScanResults stage=db_create_error trace_id=%s request_id=%s error=%v elapsed=%s",
			traceID, requestID, err, time.Since(start),
		)
		return nil, fmt.Errorf("failed to save scan results")
	}

	log.Printf(
		"op=SaveScanResults stage=success trace_id=%s request_id=%s storage_id=%s elapsed=%s",
		traceID, requestID, scanResult.ID.Hex(), time.Since(start),
	)

	return scanResult, nil
}

func ScanResourcesByIDs(ids []string, resourceType ResourceType, includePRs, includeDiscussion bool) ([]models.Finding, int, error) {
	start := time.Now()
	traceID := uuid.New().String()

	if len(ids) == 0 {
		return nil, 0, fmt.Errorf("no %s found for this organization", resourceType)
	}

	log.Printf(
		"op=ScanResourcesByIDs stage=start trace_id=%s resource_type=%s ids=%d include_prs=%t include_discussion=%t",
		traceID, resourceType, len(ids), includePRs, includeDiscussion,
	)

	var findings []models.Finding
	var mu sync.Mutex
	var scanned int

	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, 8)

	for _, id := range ids {
		rid := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			local := time.Now()
			rtrace := uuid.New().String()

			aiReq, _, err := FetchSingleResource(rid, resourceType, includePRs, includeDiscussion)
			
			if err != nil {
				log.Printf(
					"op=ScanResourcesByIDs stage=fetch_single_error trace_id=%s resource_type=%s resource_id=%s error=%v elapsed=%s",
					rtrace, resourceType, rid, err, time.Since(local),
				)
				return
			}

			fs := ScanAIRequest(*aiReq, SecretConfig, string(resourceType), rid)

			mu.Lock()
			findings = append(findings, fs...)
			scanned++
			mu.Unlock()

			log.Printf(
				"op=ScanResourcesByIDs stage=scan_done trace_id=%s resource_type=%s resource_id=%s findings=%d elapsed=%s",
				rtrace, resourceType, rid, len(fs), time.Since(local),
			)
		}()
	}

	wg.Wait()

	if scanned == 0 {
		return nil, 0, fmt.Errorf("no %s found for this organization", resourceType)
	}

	log.Printf(
		"op=ScanResourcesByIDs stage=success trace_id=%s resource_type=%s scanned=%d findings=%d total_elapsed=%s",
		traceID, resourceType, scanned, len(findings), time.Since(start),
	)

	return findings, scanned, nil
}


func FetchSingleResource(
	resourceID string,
	resourceType ResourceType,
	includePRs, includeDiscussion bool,
) (*models.AI_REQUEST, map[string]interface{}, error) {
	start := time.Now()
	traceID := uuid.New().String()

	url := fmt.Sprintf("https://huggingface.co/api/%s/%s", resourceType, resourceID)
	client := SharedHTTPClient()

	log.Printf(
		"op=FetchSingleResource stage=start trace_id=%s resource_type=%s resource_id=%s url=%s include_prs=%t include_discussion=%t",
		traceID, resourceType, resourceID, url, includePRs, includeDiscussion,
	)

	resp, err := client.Get(url)
	if err != nil {
		log.Printf(
			"op=FetchSingleResource stage=http_get_error trace_id=%s resource_type=%s resource_id=%s error=%v elapsed=%s",
			traceID, resourceType, resourceID, err, time.Since(start),
		)
		return nil, nil, fmt.Errorf("failed to fetch %s", resourceType)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf(
			"op=FetchSingleResource stage=not_ok trace_id=%s resource_type=%s resource_id=%s status=%d elapsed=%s",
			traceID, resourceType, resourceID, resp.StatusCode, time.Since(start),
		)
		return nil, nil, fmt.Errorf("%s not found", resourceType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf(
			"op=FetchSingleResource stage=read_error trace_id=%s resource_type=%s resource_id=%s error=%v elapsed=%s",
			traceID, resourceType, resourceID, err, time.Since(start),
		)
		return nil, nil, fmt.Errorf("failed to read response")
	}

	var resourceData map[string]interface{}
	if err := json.Unmarshal(body, &resourceData); err != nil {
		log.Printf(
			"op=FetchSingleResource stage=json_unmarshal_error trace_id=%s resource_type=%s resource_id=%s error=%v elapsed=%s",
			traceID, resourceType, resourceID, err, time.Since(start),
		)
		return nil, nil, fmt.Errorf("failed to parse response")
	}

	aiRequest := &models.AI_REQUEST{
		RequestID:    uuid.New().String(),
		ResourceType: string(resourceType),
		ResourceID:   resourceID,
		Siblings:     []models.SIBLING{},
		Discussions:  []models.DISCUSSION{},
	}

	if siblings, ok := resourceData["siblings"].([]interface{}); ok {
		aiRequest.Siblings = FetchFilesFromSiblings(resourceID, siblings)
	}

	if includePRs || includeDiscussion {
		discussions, _ := FetchDiscussions(resourceID, string(resourceType), includePRs, includeDiscussion)
		aiRequest.Discussions = discussions
	}

	log.Printf(
		"op=FetchSingleResource stage=success trace_id=%s resource_type=%s resource_id=%s files=%d discussions=%d elapsed=%s",
		traceID, resourceType, resourceID, len(aiRequest.Siblings), len(aiRequest.Discussions), time.Since(start),
	)

	return aiRequest, resourceData, nil
}
