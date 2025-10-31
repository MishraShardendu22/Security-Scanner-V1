package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/MishraShardendu22/Scanner/controller/discussion"
	"github.com/MishraShardendu22/Scanner/models"
	"github.com/MishraShardendu22/Scanner/util"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kamva/mgm/v3"
)

var fetchHTTPClient = util.SharedHTTPClient()

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

func FetchPRs(c *fiber.Ctx) error {
	resourceType := c.Params("type")
	resourceID := c.Params("id")
	if resourceType == "" || resourceID == "" {
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Resource type and ID are required", nil, "")
	}

	start := time.Now()
	requestID := uuid.New().String()

	log.Printf(
		"op=FetchPRs stage=start request_id=%s method=%s path=%s resource_type=%s resource_id=%s ip=%s user_agent=%q",
		requestID, c.Method(), c.OriginalURL(), resourceType, resourceID, c.IP(), c.Get("User-Agent"),
	)

	aiRequest, err := discussion.FetchAndSaveDiscussionsByType(resourceType, resourceID, "pr")
	if err != nil {
		log.Printf(
			"op=FetchPRs stage=fetch_error request_id=%s resource_type=%s resource_id=%s error=%v elapsed=%s",
			requestID, resourceType, resourceID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
	}

	log.Printf(
		"op=FetchPRs stage=success request_id=%s resource_type=%s resource_id=%s count=%d total_elapsed=%s",
		requestID, resourceType, resourceID, len(aiRequest.Discussions), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "PRs fetched successfully", map[string]interface{}{
		"request_id": aiRequest.RequestID,
		"count":      len(aiRequest.Discussions),
		"prs":        aiRequest.Discussions,
	}, "")
}

func FetchModel(c *fiber.Ctx) error {
	modelID := c.Params("modelId")
	if modelID == "" {
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Model ID is required", nil, "")
	}

	start := time.Now()
	requestID := uuid.New().String()
	url := fmt.Sprintf("https://huggingface.co/api/models/%s", modelID)

	log.Printf(
		"op=FetchModel stage=start request_id=%s method=%s path=%s model_id=%s ip=%s user_agent=%q url=%s",
		requestID, c.Method(), c.OriginalURL(), modelID, c.IP(), c.Get("User-Agent"), url,
	)

	resp, err := fetchHTTPClient.Get(url)
	if err != nil {
		log.Printf(
			"op=FetchModel stage=http_get_error request_id=%s model_id=%s url=%s error=%v elapsed=%s",
			requestID, modelID, url, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to fetch model", nil, "")
	}
	defer resp.Body.Close()

	log.Printf(
		"op=FetchModel stage=got_response request_id=%s model_id=%s status=%d content_length=%q elapsed=%s",
		requestID, modelID, resp.StatusCode, resp.Header.Get("Content-Length"), time.Since(start),
	)

	if resp.StatusCode != http.StatusOK {
		log.Printf(
			"op=FetchModel stage=not_ok request_id=%s model_id=%s status=%d elapsed=%s",
			requestID, modelID, resp.StatusCode, time.Since(start),
		)
		return util.ResponseAPI(c, resp.StatusCode, "Model not found", nil, "")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf(
			"op=FetchModel stage=read_error request_id=%s model_id=%s error=%v elapsed=%s",
			requestID, modelID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to read response", nil, "")
	}
	log.Printf(
		"op=FetchModel stage=read_ok request_id=%s model_id=%s bytes=%d elapsed=%s",
		requestID, modelID, len(body), time.Since(start),
	)

	var modelData map[string]interface{}
	if err := json.Unmarshal(body, &modelData); err != nil {
		log.Printf(
			"op=FetchModel stage=json_unmarshal_error request_id=%s model_id=%s error=%v elapsed=%s",
			requestID, modelID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to parse response", nil, "")
	}

	// basically url modify kar raha hai for the includeDiscussion
	includePRs, includeDiscussion := util.ParseIncludeFlags(c)
	log.Printf(
		"op=FetchModel stage=parse_flags request_id=%s model_id=%s include_prs=%t include_discussion=%t elapsed=%s",
		requestID, modelID, includePRs, includeDiscussion, time.Since(start),
	)

	aiRequest := &models.AI_REQUEST{
		RequestID:    requestID,
		ResourceType: "models",
		ResourceID:   modelID,
		Siblings:     []models.SIBLING{},
		Discussions:  []models.DISCUSSION{},
	}

	if siblings, ok := modelData["siblings"].([]interface{}); ok {
		log.Printf(
			"op=FetchModel stage=fetch_siblings request_id=%s model_id=%s sibling_candidates=%d",
			requestID, modelID, len(siblings),
		)
		aiRequest.Siblings = util.FetchFilesFromSiblings(modelID, siblings)
		log.Printf(
			"op=FetchModel stage=fetch_siblings_done request_id=%s model_id=%s files=%d",
			requestID, modelID, len(aiRequest.Siblings),
		)
	}

	if includePRs || includeDiscussion {
		log.Printf(
			"op=FetchModel stage=fetch_discussions_start request_id=%s model_id=%s include_prs=%t include_discussion=%t",
			requestID, modelID, includePRs, includeDiscussion,
		)
		discussions, _ := util.FetchDiscussions(modelID, "models", includePRs, includeDiscussion)
		aiRequest.Discussions = discussions
		log.Printf(
			"op=FetchModel stage=fetch_discussions_done request_id=%s model_id=%s discussions=%d",
			requestID, modelID, len(discussions),
		)
	}

	if err := mgm.Coll(aiRequest).Create(aiRequest); err != nil {
		log.Printf(
			"op=FetchModel stage=db_create_error request_id=%s model_id=%s error=%v elapsed=%s",
			requestID, modelID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to save to database", nil, "")
	}

	log.Printf(
		"op=FetchModel stage=success request_id=%s model_id=%s files=%d discussions=%d total_elapsed=%s",
		requestID, modelID, len(aiRequest.Siblings), len(aiRequest.Discussions), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Model fetched successfully", map[string]interface{}{
		"request_id":  aiRequest.RequestID,
		"model":       modelData,
		"siblings":    aiRequest.Siblings,
		"discussions": aiRequest.Discussions,
	}, "")
}

func FetchSpace(c *fiber.Ctx) error {
	spaceID := c.Params("spaceId")
	if spaceID == "" {
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Space ID is required", nil, "")
	}

	start := time.Now()
	requestID := uuid.New().String()
	url := fmt.Sprintf("https://huggingface.co/api/spaces/%s", spaceID)

	log.Printf(
		"op=FetchSpace stage=start request_id=%s method=%s path=%s space_id=%s ip=%s user_agent=%q url=%s",
		requestID, c.Method(), c.OriginalURL(), spaceID, c.IP(), c.Get("User-Agent"), url,
	)

	resp, err := fetchHTTPClient.Get(url)
	if err != nil {
		log.Printf(
			"op=FetchSpace stage=http_get_error request_id=%s space_id=%s url=%s error=%v elapsed=%s",
			requestID, spaceID, url, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to fetch space", nil, "")
	}
	defer resp.Body.Close()

	log.Printf(
		"op=FetchSpace stage=got_response request_id=%s space_id=%s status=%d content_length=%q elapsed=%s",
		requestID, spaceID, resp.StatusCode, resp.Header.Get("Content-Length"), time.Since(start),
	)

	if resp.StatusCode != http.StatusOK {
		log.Printf(
			"op=FetchSpace stage=not_ok request_id=%s space_id=%s status=%d elapsed=%s",
			requestID, spaceID, resp.StatusCode, time.Since(start),
		)
		return util.ResponseAPI(c, resp.StatusCode, "Space not found", nil, "")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf(
			"op=FetchSpace stage=read_error request_id=%s space_id=%s error=%v elapsed=%s",
			requestID, spaceID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to read response", nil, "")
	}
	log.Printf(
		"op=FetchSpace stage=read_ok request_id=%s space_id=%s bytes=%d elapsed=%s",
		requestID, spaceID, len(body), time.Since(start),
	)

	var spaceData map[string]interface{}
	if err := json.Unmarshal(body, &spaceData); err != nil {
		log.Printf(
			"op=FetchSpace stage=json_unmarshal_error request_id=%s space_id=%s error=%v elapsed=%s",
			requestID, spaceID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to parse response", nil, "")
	}

	includePRs, includeDiscussion := util.ParseIncludeFlags(c)
	log.Printf(
		"op=FetchSpace stage=parse_flags request_id=%s space_id=%s include_prs=%t include_discussion=%t elapsed=%s",
		requestID, spaceID, includePRs, includeDiscussion, time.Since(start),
	)

	aiRequest := &models.AI_REQUEST{
		RequestID:    requestID,
		ResourceType: "spaces",
		ResourceID:   spaceID,
		Siblings:     []models.SIBLING{},
		Discussions:  []models.DISCUSSION{},
	}

	if siblings, ok := spaceData["siblings"].([]interface{}); ok {

		log.Printf(
			"op=FetchSpace stage=fetch_siblings request_id=%s space_id=%s sibling_candidates=%d",
			requestID, spaceID, len(siblings),
		)
		aiRequest.Siblings = util.FetchFilesFromSiblings(spaceID, siblings)
		log.Printf(
			"op=FetchSpace stage=fetch_siblings_done request_id=%s space_id=%s files=%d",
			requestID, spaceID, len(aiRequest.Siblings),
		)
	}

	if includePRs || includeDiscussion {
		log.Printf(
			"op=FetchSpace stage=fetch_discussions_start request_id=%s space_id=%s include_prs=%t include_discussion=%t",
			requestID, spaceID, includePRs, includeDiscussion,
		)
		discussions, _ := util.FetchDiscussions(spaceID, "spaces", includePRs, includeDiscussion)
		aiRequest.Discussions = discussions
		log.Printf(
			"op=FetchSpace stage=fetch_discussions_done request_id=%s space_id=%s discussions=%d",
			requestID, spaceID, len(discussions),
		)

	}

	if err := mgm.Coll(aiRequest).Create(aiRequest); err != nil {
		log.Printf(
			"op=FetchSpace stage=db_create_error request_id=%s space_id=%s error=%v elapsed=%s",
			requestID, spaceID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to save to database", nil, "")
	}

	log.Printf(
		"op=FetchSpace stage=success request_id=%s space_id=%s files=%d discussions=%d total_elapsed=%s",
		requestID, spaceID, len(aiRequest.Siblings), len(aiRequest.Discussions), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Space fetched successfully", map[string]interface{}{
		"request_id":  aiRequest.RequestID,
		"space":       spaceData,
		"siblings":    aiRequest.Siblings,
		"discussions": aiRequest.Discussions,
	}, "")
}

func FetchDataset(c *fiber.Ctx) error {
	datasetID := c.Params("datasetId")
	if datasetID == "" {
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Dataset ID is required", nil, "")
	}

	start := time.Now()
	requestID := uuid.New().String()
	url := fmt.Sprintf("https://huggingface.co/api/datasets/%s", datasetID)

	log.Printf(
		"op=FetchDataset stage=start request_id=%s method=%s path=%s dataset_id=%s ip=%s user_agent=%q url=%s",
		requestID, c.Method(), c.OriginalURL(), datasetID, c.IP(), c.Get("User-Agent"), url,
	)

	resp, err := fetchHTTPClient.Get(url)
	if err != nil {
		log.Printf(
			"op=FetchDataset stage=http_get_error request_id=%s dataset_id=%s url=%s error=%v elapsed=%s",
			requestID, datasetID, url, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to fetch dataset", nil, "")
	}
	defer resp.Body.Close()

	log.Printf(
		"op=FetchDataset stage=got_response request_id=%s dataset_id=%s status=%d content_length=%q elapsed=%s",
		requestID, datasetID, resp.StatusCode, resp.Header.Get("Content-Length"), time.Since(start),
	)

	if resp.StatusCode != http.StatusOK {
		log.Printf(
			"op=FetchDataset stage=not_ok request_id=%s dataset_id=%s status=%d elapsed=%s",
			requestID, datasetID, resp.StatusCode, time.Since(start),
		)
		return util.ResponseAPI(c, resp.StatusCode, "Dataset not found", nil, "")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf(
			"op=FetchDataset stage=read_error request_id=%s dataset_id=%s error=%v elapsed=%s",
			requestID, datasetID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to read response", nil, "")
	}
	log.Printf(
		"op=FetchDataset stage=read_ok request_id=%s dataset_id=%s bytes=%d elapsed=%s",
		requestID, datasetID, len(body), time.Since(start),
	)

	var datasetData map[string]interface{}
	if err := json.Unmarshal(body, &datasetData); err != nil {
		log.Printf(
			"op=FetchDataset stage=json_unmarshal_error request_id=%s dataset_id=%s error=%v elapsed=%s",
			requestID, datasetID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to parse response", nil, "")
	}

	includePRs, includeDiscussion := util.ParseIncludeFlags(c)
	log.Printf(
		"op=FetchDataset stage=parse_flags request_id=%s dataset_id=%s include_prs=%t include_discussion=%t elapsed=%s",
		requestID, datasetID, includePRs, includeDiscussion, time.Since(start),
	)

	aiRequest := &models.AI_REQUEST{
		RequestID:    requestID,
		ResourceType: "datasets",
		ResourceID:   datasetID,
		Siblings:     []models.SIBLING{},
		Discussions:  []models.DISCUSSION{},
	}

	if siblings, ok := datasetData["siblings"].([]interface{}); ok {
		log.Printf(
			"op=FetchDataset stage=fetch_siblings request_id=%s dataset_id=%s sibling_candidates=%d",
			requestID, datasetID, len(siblings),
		)
		aiRequest.Siblings = util.FetchFilesFromSiblings(datasetID, siblings)
		log.Printf(
			"op=FetchDataset stage=fetch_siblings_done request_id=%s dataset_id=%s files=%d",
			requestID, datasetID, len(aiRequest.Siblings),
		)
	}

	if includePRs || includeDiscussion {
		log.Printf(
			"op=FetchDataset stage=fetch_discussions_start request_id=%s dataset_id=%s include_prs=%t include_discussion=%t",
			requestID, datasetID, includePRs, includeDiscussion,
		)
		discussions, _ := util.FetchDiscussions(datasetID, "datasets", includePRs, includeDiscussion)
		aiRequest.Discussions = discussions
		log.Printf(
			"op=FetchDataset stage=fetch_discussions_done request_id=%s dataset_id=%s discussions=%d",
			requestID, datasetID, len(discussions),
		)
	}

	if err := mgm.Coll(aiRequest).Create(aiRequest); err != nil {
		log.Printf(
			"op=FetchDataset stage=db_create_error request_id=%s dataset_id=%s error=%v elapsed=%s",
			requestID, datasetID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, "Failed to save to database", nil, "")
	}

	log.Printf(
		"op=FetchDataset stage=success request_id=%s dataset_id=%s files=%d discussions=%d total_elapsed=%s",
		requestID, datasetID, len(aiRequest.Siblings), len(aiRequest.Discussions), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Dataset fetched successfully", map[string]interface{}{
		"request_id":  aiRequest.RequestID,
		"dataset":     datasetData,
		"siblings":    aiRequest.Siblings,
		"discussions": aiRequest.Discussions,
	}, "")
}

func FetchDiscussions(c *fiber.Ctx) error {
	resourceType := c.Params("type")
	resourceID := c.Params("id")
	if resourceType == "" || resourceID == "" {
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Resource type and ID are required", nil, "")
	}

	start := time.Now()
	requestID := uuid.New().String()

	log.Printf(
		"op=FetchDiscussions stage=start request_id=%s method=%s path=%s resource_type=%s resource_id=%s ip=%s user_agent=%q",
		requestID, c.Method(), c.OriginalURL(), resourceType, resourceID, c.IP(), c.Get("User-Agent"),
	)

	// discissons can be of two types: "pr" or "discussion",
	// this basically fetches them and saves them in db in a way that we know
	// if pr is fetched or discussion is fetched
	// if both, pr and discussion are needed, we call this twice
	aiRequest, err := discussion.FetchAndSaveDiscussionsByType(resourceType, resourceID, "discussion")
	if err != nil {
		log.Printf(
			"op=FetchDiscussions stage=fetch_error request_id=%s resource_type=%s resource_id=%s error=%v elapsed=%s",
			requestID, resourceType, resourceID, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
	}

	log.Printf(
		"op=FetchDiscussions stage=success request_id=%s resource_type=%s resource_id=%s count=%d total_elapsed=%s",
		requestID, resourceType, resourceID, len(aiRequest.Discussions), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, "Discussions fetched successfully", map[string]interface{}{
		"request_id":  aiRequest.RequestID,
		"count":       len(aiRequest.Discussions),
		"discussions": aiRequest.Discussions,
	}, "")
}
