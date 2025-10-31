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
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

type ResourceType string

const (
	ResourceTypeModel   ResourceType = "models"
	ResourceTypeSpace   ResourceType = "spaces"
	ResourceTypeDataset ResourceType = "datasets"
)

// fetch an org resource models, sapces or dataset tahts it.
func FetchOrgResources(
	c *fiber.Ctx,
	org string,
	resourceType ResourceType,
	includePRs, includeDiscussion bool,
) ([]map[string]interface{}, []string, error) {
	start := time.Now()
	requestID := uuid.New().String()

	url := fmt.Sprintf("https://huggingface.co/api/%s?author=%s&full=true", resourceType, org)
	httpClient := SharedHTTPClient()

	log.Printf(
		"op=FetchOrgResources stage=start request_id=%s method=%s path=%s org=%s resource_type=%s ip=%s user_agent=%q include_prs=%t include_discussion=%t url=%s",
		requestID, c.Method(), c.OriginalURL(), org, resourceType, c.IP(), c.Get("User-Agent"), includePRs, includeDiscussion, url,
	)

	// ye basically return karta hai array of models, spaces ya datasets
	// uss array se ham nikal sakte ya zaruuri details
	// eg - siblings (readable files), readme, etc.
	var resp *http.Response
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err = httpClient.Get(url)
		if err != nil {
			log.Printf(
				"op=FetchOrgResources stage=http_get_error request_id=%s org=%s resource_type=%s url=%s error=%v attempt=%d elapsed=%s",
				requestID, org, resourceType, url, err, attempt, time.Since(start),
			)
			time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			break
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			log.Printf(
				"op=FetchOrgResources stage=retryable_status request_id=%s org=%s resource_type=%s status=%d attempt=%d",
				requestID, org, resourceType, resp.StatusCode, attempt,
			)
			time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch %s", resourceType)
	}
	defer resp.Body.Close()

	log.Printf(
		"op=FetchOrgResources stage=got_response request_id=%s org=%s resource_type=%s status=%d content_length=%q elapsed=%s",
		requestID, org, resourceType, resp.StatusCode, resp.Header.Get("Content-Length"), time.Since(start),
	)

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("failed to fetch %s: status=%d", resourceType, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf(
			"op=FetchOrgResources stage=read_error request_id=%s org=%s resource_type=%s error=%v elapsed=%s",
			requestID, org, resourceType, err, time.Since(start),
		)
		return nil, nil, fmt.Errorf("failed to read response")
	}

	log.Printf(
		"op=FetchOrgResources stage=read_ok request_id=%s org=%s resource_type=%s bytes=%d",
		requestID, org, resourceType, len(body),
	)

	var resourcesData []map[string]interface{}
	if err := json.Unmarshal(body, &resourcesData); err != nil {
		log.Printf(
			"op=FetchOrgResources stage=json_unmarshal_error request_id=%s org=%s resource_type=%s error=%v elapsed=%s",
			requestID, org, resourceType, err, time.Since(start),
		)
		return nil, nil, fmt.Errorf("failed to parse response")
	}

	log.Printf(
		"op=FetchOrgResources stage=parsed request_id=%s org=%s resource_type=%s count=%d",
		requestID, org, resourceType, len(resourcesData),
	)

	var saved []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 20)
	total := len(resourcesData)

	// yaha ham un resources ko save kar rahe hai jo hamare db me nahi hai
	// taaki scan ke time pe hamare paas wo resources available ho jinhe scan karna hai
	// resource that we scanned can be models, spaces or datasets
	// models will give array of models for that org
	// spaces will give array of spaces for that org
	// datasets will give array of datasets for that org
	seen := make(map[string]struct{}, total)
	for idx, resourceData := range resourcesData {
		resourceID, ok := resourceData["id"].(string)
		if !ok {
			log.Printf(
				"op=FetchOrgResources stage=skip_missing_id request_id=%s org=%s resource_type=%s index=%d",
				requestID, org, resourceType, idx,
			)
			continue
		}
		if _, dup := seen[resourceID]; dup {
			continue
		}
		seen[resourceID] = struct{}{}

		wg.Add(1)
		go func(id string, index int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			localStart := time.Now()
			log.Printf(
				"op=FetchOrgResources stage=save_start request_id=%s org=%s resource_type=%s index=%d total=%d id=%s",
				requestID, org, resourceType, index+1, total, id,
			)

			switch resourceType {
			case ResourceTypeModel:
				model := &models.AI_Models{
					BaseAI: models.BaseAI{
						Org:               org,
						IncludePRS:        includePRs,
						IncludeDiscussion: includeDiscussion,
					},
					Model_ID: id,
				}
				if err := mgm.Coll(model).Create(model); err == nil {
					mu.Lock()
					saved = append(saved, id)
					mu.Unlock()
					log.Printf(
						"op=FetchOrgResources stage=save_done request_id=%s resource_type=%s id=%s elapsed=%s",
						requestID, resourceType, id, time.Since(localStart),
					)
				} else {
					if mongo.IsDuplicateKeyError(err) {
						log.Printf(
							"op=FetchOrgResources stage=duplicate request_id=%s resource_type=%s id=%s elapsed=%s",
							requestID, resourceType, id, time.Since(localStart),
						)
					} else {
						log.Printf(
							"op=FetchOrgResources stage=db_create_error request_id=%s resource_type=%s id=%s error=%v elapsed=%s",
							requestID, resourceType, id, err, time.Since(localStart),
						)
					}
				}
			case ResourceTypeDataset:
				model := &models.AI_DATASETS{
					BaseAI: models.BaseAI{
						Org:               org,
						IncludePRS:        includePRs,
						IncludeDiscussion: includeDiscussion,
					},
					Dataset_ID: id,
				}
				if err := mgm.Coll(model).Create(model); err == nil {
					mu.Lock()
					saved = append(saved, id)
					mu.Unlock()
					log.Printf(
						"op=FetchOrgResources stage=save_done request_id=%s resource_type=%s id=%s elapsed=%s",
						requestID, resourceType, id, time.Since(localStart),
					)
				} else {
					if mongo.IsDuplicateKeyError(err) {
						log.Printf(
							"op=FetchOrgResources stage=duplicate request_id=%s resource_type=%s id=%s elapsed=%s",
							requestID, resourceType, id, time.Since(localStart),
						)
					} else {
						log.Printf(
							"op=FetchOrgResources stage=db_create_error request_id=%s resource_type=%s id=%s error=%v elapsed=%s",
							requestID, resourceType, id, err, time.Since(localStart),
						)
					}
				}
			case ResourceTypeSpace:
				model := &models.AI_SPACES{
					BaseAI: models.BaseAI{
						Org:               org,
						IncludePRS:        includePRs,
						IncludeDiscussion: includeDiscussion,
					},
					Space_ID: id,
				}
				if err := mgm.Coll(model).Create(model); err == nil {
					mu.Lock()
					saved = append(saved, id)
					mu.Unlock()
					log.Printf(
						"op=FetchOrgResources stage=save_done request_id=%s resource_type=%s id=%s elapsed=%s",
						requestID, resourceType, id, time.Since(localStart),
					)
				} else {
					if mongo.IsDuplicateKeyError(err) {
						log.Printf(
							"op=FetchOrgResources stage=duplicate request_id=%s resource_type=%s id=%s elapsed=%s",
							requestID, resourceType, id, time.Since(localStart),
						)
					} else {
						log.Printf(
							"op=FetchOrgResources stage=db_create_error request_id=%s resource_type=%s id=%s error=%v elapsed=%s",
							requestID, resourceType, id, err, time.Since(localStart),
						)
					}
				}
			}
		}(resourceID, idx)
	}
	wg.Wait()

	log.Printf(
		"op=FetchOrgResources stage=success request_id=%s org=%s resource_type=%s saved=%d total=%d total_elapsed=%s",
		requestID, org, resourceType, len(saved), len(resourcesData), time.Since(start),
	)

	return resourcesData, saved, nil
}
