package discussion

import (
	"fmt"

	"github.com/MishraShardendu22/Scanner/models"
	"github.com/MishraShardendu22/Scanner/util"
	"github.com/google/uuid"
	"github.com/kamva/mgm/v3"
)

// discissons can be of two types: "pr" or "discussion",
// this basically fetches them and saves them in db in a way that we know
// if pr is fetched or discussion is fetched
func FetchAndSaveDiscussionsByType(resourceType, resourceID, discussionType string) (*models.AI_REQUEST, error) {
	url := fmt.Sprintf("https://huggingface.co/api/%s/%s/discussions?types=%s&status=all", resourceType, resourceID, discussionType)

	discussions, err := util.GetDiscussionsFromURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s", discussionType)
	}

	aiRequest := &models.AI_REQUEST{
		RequestID:   uuid.New().String(),
		Siblings:    []models.SIBLING{},
		Discussions: discussions,
	}

	if err := mgm.Coll(aiRequest).Create(aiRequest); err != nil {
		return nil, fmt.Errorf("failed to save to database")
	}

	return aiRequest, nil
}
