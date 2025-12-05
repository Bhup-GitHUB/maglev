package restapi

import (
	"net/http"
	"strconv"

	"maglev.onebusaway.org/gtfsdb"
	"maglev.onebusaway.org/internal/models"
	"maglev.onebusaway.org/internal/utils"
)

const defaultMaxCount = 20

func (api *RestAPI) searchRouteHandler(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()

	input := queryParams.Get("input")
	if input == "" {
		fieldErrors := map[string][]string{
			"input": {"input parameter is required"},
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
		maxCount = parsed
	}

	ctx := r.Context()

	if ctx.Err() != nil {
		api.serverErrorResponse(w, r, ctx.Err())
		return
	}

	searchTerm := input + "*"
	routes, err := api.GtfsManager.GtfsDB.Queries.SearchRoutesByName(ctx, gtfsdb.SearchRoutesByNameParams{
		SearchTerm: searchTerm,
		MaxCount:   maxCount,
	})
	if err != nil {
		api.serverErrorResponse(w, r, err)
		return
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

	response := models.NewListResponse(routesList, references)
	api.sendResponse(w, r, response)
}
