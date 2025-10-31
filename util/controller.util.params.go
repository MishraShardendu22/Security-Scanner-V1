package util

import "github.com/gofiber/fiber/v2"

func ParseIncludeFlags(c *fiber.Ctx) (includePRs bool, includeDiscussion bool) {
	includePRs = c.Query("include_prs", "false") == "true"
	includeDiscussion = c.Query("include_discussion", "false") == "true"
	return
}

func ParsePagination(c *fiber.Ctx) (page int, limit int) {
	page = c.QueryInt("page", 1)
	limit = c.QueryInt("limit", 10)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	return
}

// Q. What is the use case of this file?
// A. Pagination control karne ke liye hai 2nd fucntion
// and pehle function just includes flags as query params.
// Basically kya hai na ho sakta hai
// we send query params like ?include_prs=true&include_discussion=false
// bass is process ko standardize karne ke liye ye file hai.
