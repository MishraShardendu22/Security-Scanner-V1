package controller

import (
	"fmt"
	"log"
	"time"

	"github.com/MishraShardendu22/Scanner/util"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

func FetchOrgModels(c *fiber.Ctx) error {
	org := c.Params("org")
	if org == "" {
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Organization name is required", nil, "")
	}

	start := time.Now()
	requestID := uuid.New().String()
	includePRs, includeDiscussion := util.ParseIncludeFlags(c)

	log.Printf(
		"op=FetchOrgModels stage=start request_id=%s method=%s path=%s org=%s ip=%s user_agent=%q include_prs=%t include_discussion=%t",
		requestID, c.Method(), c.OriginalURL(), org, c.IP(), c.Get("User-Agent"), includePRs, includeDiscussion,
	)

	modelsData, savedModels, err := util.FetchOrgResources(c, org, util.ResourceTypeModel, includePRs, includeDiscussion)
	if err != nil {
		log.Printf(
			"op=FetchOrgModels stage=fetch_error request_id=%s org=%s error=%v elapsed=%s",
			requestID, org, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
	}

	log.Printf(
		"op=FetchOrgModels stage=success request_id=%s org=%s count=%d saved=%d total_elapsed=%s",
		requestID, org, len(modelsData), len(savedModels), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, fmt.Sprintf("Fetched %d models for organization %s", len(modelsData), org), map[string]interface{}{
		"organization": org,
		"count":        len(modelsData),
		"saved_count":  len(savedModels),
		"models":       modelsData,
	}, "")
}

func FetchOrgDatasets(c *fiber.Ctx) error {
	org := c.Params("org")
	if org == "" {
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Organization name is required", nil, "")
	}

	start := time.Now()
	requestID := uuid.New().String()
	includePRs, includeDiscussion := util.ParseIncludeFlags(c)

	log.Printf(
		"op=FetchOrgDatasets stage=start request_id=%s method=%s path=%s org=%s ip=%s user_agent=%q include_prs=%t include_discussion=%t",
		requestID, c.Method(), c.OriginalURL(), org, c.IP(), c.Get("User-Agent"), includePRs, includeDiscussion,
	)

	datasetsData, savedDatasets, err := util.FetchOrgResources(c, org, util.ResourceTypeDataset, includePRs, includeDiscussion)
	if err != nil {
		log.Printf(
			"op=FetchOrgDatasets stage=fetch_error request_id=%s org=%s error=%v elapsed=%s",
			requestID, org, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
	}

	log.Printf(
		"op=FetchOrgDatasets stage=success request_id=%s org=%s count=%d saved=%d total_elapsed=%s",
		requestID, org, len(datasetsData), len(savedDatasets), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, fmt.Sprintf("Fetched %d datasets for organization %s", len(datasetsData), org), map[string]interface{}{
		"organization": org,
		"count":        len(datasetsData),
		"saved_count":  len(savedDatasets),
		"datasets":     datasetsData,
	}, "")
}

func FetchOrgSpaces(c *fiber.Ctx) error {
	org := c.Params("org")
	if org == "" {
		return util.ResponseAPI(c, fiber.StatusBadRequest, "Organization name is required", nil, "")
	}

	start := time.Now()
	requestID := uuid.New().String()
	includePRs, includeDiscussion := util.ParseIncludeFlags(c)

	log.Printf(
		"op=FetchOrgSpaces stage=start request_id=%s method=%s path=%s org=%s ip=%s user_agent=%q include_prs=%t include_discussion=%t",
		requestID, c.Method(), c.OriginalURL(), org, c.IP(), c.Get("User-Agent"), includePRs, includeDiscussion,
	)

	spacesData, savedSpaces, err := util.FetchOrgResources(c, org, util.ResourceTypeSpace, includePRs, includeDiscussion)
	if err != nil {
		log.Printf(
			"op=FetchOrgSpaces stage=fetch_error request_id=%s org=%s error=%v elapsed=%s",
			requestID, org, err, time.Since(start),
		)
		return util.ResponseAPI(c, fiber.StatusInternalServerError, err.Error(), nil, "")
	}

	log.Printf(
		"op=FetchOrgSpaces stage=success request_id=%s org=%s count=%d saved=%d total_elapsed=%s",
		requestID, org, len(spacesData), len(savedSpaces), time.Since(start),
	)

	return util.ResponseAPI(c, fiber.StatusOK, fmt.Sprintf("Fetched %d spaces for organization %s", len(spacesData), org), map[string]interface{}{
		"organization": org,
		"count":        len(spacesData),
		"saved_count":  len(savedSpaces),
		"spaces":       spacesData,
	}, "")
}
