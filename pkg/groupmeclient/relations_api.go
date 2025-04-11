package groupmeclient

import (
	"context"
	"net/http"
	"time"
)

// GroupMe documentation does not cover this "v4" endpoint
// I don't know how else you're supposed to get your contact list
const (
	GroupMeAPIBaseV4         = GroupMeAPIPath + "v4"
	relationshipEndpointRoot = "/relationships"
)

// RelationsQuery defineds the optional URL parameters for IndexRelations
type RelationsQuery struct {
	// Time since relation was last updated. Odd choice but ok?
	Since string `json:"since"`
}

// IndexRelations - Returns a paginated list of relations (users)
// sorted by "updated_at"
func (c *Client) IndexRelations(ctx context.Context, relationsQuery *RelationsQuery) ([]*User, error) {
	httpReq, err := http.NewRequest("GET", GroupMeAPIBaseV4+relationshipEndpointRoot, nil)
	if err != nil {
		return nil, err
	}

	URL := httpReq.URL
	query := URL.Query()
	query.Set("include_blocked", "true")

	if relationsQuery != nil {
		if relationsQuery.Since != "" {
			query.Set("since", relationsQuery.Since)
		}
	}

	URL.RawQuery = query.Encode()

	var resp []*User
	err = c.doWithAuthToken(ctx, httpReq, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) IndexAllRelations(ctx context.Context) ([]*User, error) {
	var resp []*User
	relationsQuery := RelationsQuery{}
	for {
		currentPageRelations, err := c.IndexRelations(ctx, &relationsQuery)
		if err != nil {
			return resp, err
		}
		resp = append(resp, currentPageRelations...)
		if len(currentPageRelations) < 200 {
			break
		}
		relationsQuery.Since = currentPageRelations[len(currentPageRelations)-1].UpdatedAt.ToTime().Format(time.RFC3339)
	}
	return resp, nil
}
