package restapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"maglev.onebusaway.org/gtfsdb"
	"maglev.onebusaway.org/internal/models"
	"maglev.onebusaway.org/internal/utils"
)

const defaultMaxCount = 20

func (api *RestAPI) searchRouteHandler(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()

	input := queryParams.Get("input")
	// Validate and sanitize input to avoid FTS syntax errors and injection
	searchTerm, err := buildRouteSearchTerm(input)
	if err != nil {
		fieldErrors := map[string][]string{
			"input": {err.Error()},
		}
		api.validationErrorResponse(w, r, fieldErrors)
		return
	}

	maxCount := int64(defaultMaxCount)
	maxCountStr := queryParams.Get("maxCount")
	if maxCountStr != "" {
		parsed, err := strconv.ParseInt(maxCountStr, 10, 64)
		if err != nil || parsed <= 0 {
			fieldErrors := map[string][]string{
				"maxCount": {"maxCount must be a positive integer"},
			}
			api.validationErrorResponse(w, r, fieldErrors)
			return
		}
		if parsed > defaultMaxCount {
			fieldErrors := map[string][]string{
				"maxCount": {fmt.Sprintf("maxCount must not exceed %d", defaultMaxCount)},
			}
			api.validationErrorResponse(w, r, fieldErrors)
			return
		}
		maxCount = parsed
	}

	ctx := r.Context()

	if ctx.Err() != nil {
		api.serverErrorResponse(w, r, ctx.Err())
		return
	}

	routes, err := api.GtfsManager.GtfsDB.Queries.SearchRoutesByName(ctx, gtfsdb.SearchRoutesByNameParams{
		SearchTerm: searchTerm,
		MaxCount:   maxCount + 1, // fetch one extra to detect truncation
	})
	if err != nil {
		api.serverErrorResponse(w, r, err)
		return
	}

	limitExceeded := int64(len(routes)) > maxCount
	if limitExceeded {
		routes = routes[:maxCount]
	}

	routesList := make([]models.Route, 0, len(routes))
	agencyIDs := make(map[string]bool)

	for _, route := range routes {
		agencyIDs[route.AgencyID] = true
		routesList = append(routesList, models.NewRoute(
			utils.FormCombinedID(route.AgencyID, route.ID),
			route.AgencyID,
			route.ShortName.String,
			route.LongName.String,
			route.Desc.String,
			models.RouteType(route.Type),
			route.Url.String,
			route.Color.String,
			route.TextColor.String,
			route.ShortName.String,
		))
	}

	agencies := utils.FilterAgencies(api.GtfsManager.GetAgencies(), agencyIDs)

	references := models.ReferencesModel{
		Agencies:   agencies,
		Routes:     []interface{}{},
		Situations: []interface{}{},
		StopTimes:  []interface{}{},
		Stops:      []models.Stop{},
		Trips:      []interface{}{},
	}

	response := models.NewOKResponse(map[string]interface{}{
		"limitExceeded": limitExceeded,
		"list":          routesList,
		"references":    references,
	})
	api.sendResponse(w, r, response)
}

// buildRouteSearchTerm sanitizes input and converts it into a safe FTS5 MATCH query.
// We quote each term and add a trailing wildcard for prefix search, joining multiple
// terms with AND to approximate the upstream behavior while avoiding operator injection.
func buildRouteSearchTerm(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", errors.New("input parameter is required")
	}

	sanitized, err := utils.ValidateAndSanitizeQuery(input)
	if err != nil {
		return "", err
	}

	terms := strings.Fields(sanitized)
	if len(terms) == 0 {
		return "", errors.New("input parameter is required")
	}

	escaped := make([]string, 0, len(terms))
	for _, term := range terms {
		// Drop quotes that would break MATCH syntax
		clean := strings.Map(func(r rune) rune {
			switch r {
			case '"', '\'':
				return -1
			default:
				return r
			}
		}, term)

		clean = strings.TrimSpace(clean)
		if clean == "" {
			continue
		}

		escaped = append(escaped, fmt.Sprintf("\"%s\"*", clean))
	}

	if len(escaped) == 0 {
		return "", errors.New("input parameter is required")
	}

	return strings.Join(escaped, " AND "), nil
}
