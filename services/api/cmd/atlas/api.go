package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type service struct {
	db      *sql.DB
	webRoot string
}
type Agent struct {
	ID      string   `json:"id"`
	Length  *float64 `json:"length,omitempty"`
	Width   *float64 `json:"width,omitempty"`
	Color   string   `json:"color,omitempty"`
	Samples []Sample `json:"samples,omitempty"`
}
type Sample struct {
	T float64 `json:"t"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Scenario struct {
	ID                    string         `json:"id"`
	DataClass             string         `json:"dataClass"`
	SourceDataset         string         `json:"sourceDataset"`
	SourceRelease         string         `json:"sourceRelease"`
	OriginalScenarioRef   *string        `json:"originalScenarioRef"`
	OriginalSceneID       *string        `json:"originalSceneId"`
	OriginalDataFile      *string        `json:"originalDataFile"`
	SourceCacheFolder     *string        `json:"sourceCacheFolder"`
	SourceScenarioIndex   *string        `json:"sourceScenarioIndex"`
	TrackIDs              []string       `json:"trackIds"`
	OriginalTrackIDs      []string       `json:"originalTrackIds"`
	InteractionStart      *float64       `json:"interactionStart"`
	InteractionEnd        *float64       `json:"interactionEnd"`
	SourcePET             *float64       `json:"sourcePet"`
	SourceIntensity       *float64       `json:"sourceIntensity"`
	AgentCount            int            `json:"agentCount"`
	AgentCountLabel       *string        `json:"agentCountLabel"`
	KeyAgentIDs           []string       `json:"keyAgentIds"`
	AVInvolvement         *string        `json:"avInvolvement"`
	SourceVehicleTypes    any            `json:"sourceVehicleTypes"`
	SourcePathCategory    *string        `json:"sourcePathCategory"`
	SourcePathRelation    *string        `json:"sourcePathRelation"`
	TrajectoriesAvailable bool           `json:"trajectoriesAvailable"`
	MapAvailable          bool           `json:"mapAvailable"`
	QualityFlags          []string       `json:"qualityFlags"`
	OriginalAnnotations   map[string]any `json:"originalAnnotations"`
	Agents                []Agent        `json:"agents,omitempty"`
	Relationship          string         `json:"relationship,omitempty"`
	Provenance            string         `json:"provenance,omitempty"`
}
type queryFilters struct {
	dataset        string
	search         string
	relationship   string
	pathCategory   string
	avInvolvement  string
	minAgents      *int
	maxAgents      *int
	petMetric      string
	petOperator    string
	petValue       *float64
	pairScope      string
	pairAgents     []pairAgentFilter
	distinctAgents bool
}
type pairAgentFilter struct {
	TurningDirection string `json:"turningDirection"`
	Priority         *bool  `json:"priority"`
}
type searchRequest struct {
	Filters struct {
		Dataset       string `json:"dataset"`
		Query         string `json:"query"`
		Relationship  string `json:"pathRelationship"`
		PathCategory  string `json:"pathCategory"`
		AVInvolvement string `json:"avInvolvement"`
		AgentCount    struct {
			Min *int `json:"min"`
			Max *int `json:"max"`
		} `json:"agentCount"`
		PET struct {
			Metric   string   `json:"metric"`
			Operator string   `json:"operator"`
			Value    *float64 `json:"value"`
		} `json:"pet"`
		InteractionPair struct {
			Scope          string            `json:"scope"`
			Agents         []pairAgentFilter `json:"agents"`
			DistinctAgents bool              `json:"distinctAgents"`
		} `json:"interactionPair"`
	} `json:"filters"`
	IncludeFixtures bool   `json:"includeFixtures"`
	Limit           int    `json:"limit"`
	Cursor          string `json:"cursor"`
	SortField       string `json:"sortField"`
	SortDirection   string `json:"sortDirection"`
}
type page struct {
	limit           int
	offset          int
	sortField       string
	sortDirection   string
	includeFixtures bool
}

func (s *service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.health)
	mux.HandleFunc("GET /v1/releases", s.releases)
	mux.HandleFunc("GET /v1/overview", s.overview)
	mux.HandleFunc("GET /v1/datasets", s.datasets)
	mux.HandleFunc("GET /v1/datasets/{id}", s.dataset)
	mux.HandleFunc("GET /v1/datasets/{id}/statistics", s.datasetStats)
	mux.HandleFunc("GET /v1/scenarios", s.listScenarios)
	mux.HandleFunc("POST /v1/scenarios/search", s.searchScenarios)
	mux.HandleFunc("POST /v1/scenarios/compare", s.compareScenarios)
	mux.HandleFunc("POST /v1/analytics/query", s.analyticsQuery)
	mux.HandleFunc("GET /v1/scenarios/{id}", s.getScenario)
	mux.HandleFunc("GET /v1/scenarios/{id}/agents", s.scenarioAgents)
	mux.HandleFunc("GET /v1/scenarios/{id}/trajectories", s.scenarioTrajectories)
	mux.HandleFunc("GET /v1/scenarios/{id}/map", s.scenarioMap)
	mux.HandleFunc("GET /v1/scenarios/{id}/metrics", s.scenarioMetrics)
	mux.HandleFunc("GET /v1/scenarios/{id}/conflicts", s.scenarioConflicts)
	mux.HandleFunc("GET /v1/scenarios/{id}/quality", s.scenarioQuality)
	mux.HandleFunc("GET /v1/relationships", s.relationships)
	mux.HandleFunc("GET /v1/quality", s.quality)
	mux.HandleFunc("GET /v1/openapi.json", s.openapi)
	mux.Handle("/", http.FileServer(http.Dir(s.webRoot)))
	return mux
}
func (s *service) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		apiError(w, 503, "index_unavailable", err.Error())
		return
	}
	var research, fixtures int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM scenarios WHERE data_class='research'").Scan(&research); err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM scenarios WHERE data_class='synthetic-fixture'").Scan(&fixtures); err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	respond(w, map[string]any{"status": "ok", "service": "traffic-conflict-atlas", "researchScenarios": research, "syntheticFixtures": fixtures})
}
func (s *service) releases(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), "SELECT release_id,source,imported_at,scenario_count,metadata_only,license_json FROM releases ORDER BY imported_at DESC,release_id")
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, source, at, license string
		var count int
		var meta int
		if err = rows.Scan(&id, &source, &at, &count, &meta, &license); err != nil {
			apiError(w, 500, "index_error", err.Error())
			return
		}
		var terms map[string]any
		_ = json.Unmarshal([]byte(license), &terms)
		out = append(out, map[string]any{"id": id, "source": source, "importedAt": at, "scenarioCount": count, "metadataOnly": meta == 1, "license": terms})
	}
	if err = rows.Err(); err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	respond(w, out)
}
func (s *service) overview(w http.ResponseWriter, r *http.Request) {
	var research, fixtures, trajectories, maps int
	err := s.db.QueryRowContext(r.Context(), "SELECT count(*),coalesce(sum(trajectories_available),0),coalesce(sum(map_available),0) FROM scenarios WHERE data_class='research'").Scan(&research, &trajectories, &maps)
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	if err = s.db.QueryRowContext(r.Context(), "SELECT count(*) FROM scenarios WHERE data_class='synthetic-fixture'").Scan(&fixtures); err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	respond(w, map[string]any{"researchDataImported": research > 0, "researchScenarioCount": research, "researchTrajectoryCount": trajectories, "researchMapCount": maps, "fixturesAvailable": fixtures, "notice": "Synthetic fixtures are excluded from research statistics."})
}
func (s *service) datasets(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT source_dataset,count(*),sum(CASE WHEN source_pet IS NOT NULL THEN 1 ELSE 0 END),sum(trajectories_available),sum(map_available),count(DISTINCT source_release) FROM scenarios WHERE data_class='research' GROUP BY source_dataset ORDER BY source_dataset`)
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id string
		var count, pet, traj, maps, releases int
		if err := rows.Scan(&id, &count, &pet, &traj, &maps, &releases); err != nil {
			apiError(w, 500, "index_error", err.Error())
			return
		}
		items = append(items, map[string]any{"id": id, "scenarioCount": count, "sourcePetCoverage": pet, "trajectoryCount": traj, "mapCount": maps, "releaseCount": releases})
	}
	respond(w, items)
}
func (s *service) dataset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var count, pet, traj, maps int
	var releaseCount int
	err := s.db.QueryRowContext(r.Context(), "SELECT count(*),coalesce(sum(source_pet IS NOT NULL),0),coalesce(sum(trajectories_available),0),coalesce(sum(map_available),0),count(DISTINCT source_release) FROM scenarios WHERE data_class='research' AND source_dataset=?", id).Scan(&count, &pet, &traj, &maps, &releaseCount)
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	if count == 0 {
		http.NotFound(w, r)
		return
	}
	respond(w, map[string]any{"id": id, "scenarioCount": count, "sourcePetCoverage": pet, "trajectoryCount": traj, "mapCount": maps, "releaseCount": releaseCount})
}
func (s *service) datasetStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var count int
	var min, max sql.NullFloat64
	err := s.db.QueryRowContext(r.Context(), "SELECT count(*),min(source_pet),max(source_pet) FROM scenarios WHERE data_class='research' AND source_dataset=?", id).Scan(&count, &min, &max)
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	if count == 0 {
		http.NotFound(w, r)
		return
	}
	respond(w, map[string]any{"dataset": id, "scenarioCount": count, "sourcePET": map[string]any{"min": nullable(min), "max": nullable(max), "coverage": countPET(r.Context(), s.db, id)}})
}
func countPET(ctx context.Context, db *sql.DB, dataset string) int {
	var n int
	_ = db.QueryRowContext(ctx, "SELECT count(source_pet) FROM scenarios WHERE data_class='research' AND source_dataset=?", dataset).Scan(&n)
	return n
}
func nullable(v sql.NullFloat64) any {
	if !v.Valid {
		return nil
	}
	return v.Float64
}

func (s *service) listScenarios(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filters := queryFilters{dataset: q.Get("dataset"), search: q.Get("q"), relationship: q.Get("pathRelationship"), pathCategory: q.Get("pathCategory"), avInvolvement: q.Get("avInvolvement")}
	if raw := q.Get("agentCountMin"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			apiError(w, 400, "invalid_filter", "agentCountMin must be a nonnegative integer")
			return
		}
		filters.minAgents = &n
	}
	if raw := q.Get("agentCountMax"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			apiError(w, 400, "invalid_filter", "agentCountMax must be a nonnegative integer")
			return
		}
		filters.maxAgents = &n
	}
	if raw := q.Get("sourcePetLt"); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || !finite(value) {
			apiError(w, 400, "invalid_filter", "sourcePetLt must be finite numeric value")
			return
		}
		filters.petMetric = "source_pet"
		filters.petOperator = "lt"
		filters.petValue = &value
	}
	pg, err := parsePage(q.Get("limit"), q.Get("cursor"), q.Get("sort"), q.Get("direction"), q.Get("includeFixtures") == "true")
	if err != nil {
		apiError(w, 400, "invalid_page", err.Error())
		return
	}
	s.runSearch(w, r, filters, pg)
}
func (s *service) searchScenarios(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		apiError(w, 400, "invalid_request", err.Error())
		return
	}
	if err := ensureEOF(dec); err != nil {
		apiError(w, 400, "invalid_request", err.Error())
		return
	}
	f := filtersFromRequest(req)
	if req.Filters.PET.Value != nil {
		f.petMetric = req.Filters.PET.Metric
		f.petOperator = req.Filters.PET.Operator
		f.petValue = req.Filters.PET.Value
	}
	if f.minAgents != nil && *f.minAgents < 0 || f.maxAgents != nil && *f.maxAgents < 0 {
		apiError(w, 400, "invalid_filter", "agent count bounds must be nonnegative")
		return
	}
	pg, err := parsePage(strconv.Itoa(defaultLimit(req.Limit)), req.Cursor, req.SortField, req.SortDirection, req.IncludeFixtures)
	if err != nil {
		apiError(w, 400, "invalid_page", err.Error())
		return
	}
	s.runSearch(w, r, f, pg)
}
func filtersFromRequest(req searchRequest) queryFilters {
	f := queryFilters{dataset: req.Filters.Dataset, search: req.Filters.Query, relationship: req.Filters.Relationship, pathCategory: req.Filters.PathCategory, avInvolvement: req.Filters.AVInvolvement, minAgents: req.Filters.AgentCount.Min, maxAgents: req.Filters.AgentCount.Max, pairScope: req.Filters.InteractionPair.Scope, pairAgents: req.Filters.InteractionPair.Agents, distinctAgents: req.Filters.InteractionPair.DistinctAgents}
	if req.Filters.PET.Value != nil {
		f.petMetric = req.Filters.PET.Metric
		f.petOperator = req.Filters.PET.Operator
		f.petValue = req.Filters.PET.Value
	}
	return f
}
func decodeSearchBody(w http.ResponseWriter, r *http.Request) (searchRequest, bool) {
	var req searchRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		apiError(w, 400, "invalid_request", err.Error())
		return req, false
	}
	if err := ensureEOF(dec); err != nil {
		apiError(w, 400, "invalid_request", err.Error())
		return req, false
	}
	return req, true
}
func (s *service) analyticsQuery(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeSearchBody(w, r)
	if !ok {
		return
	}
	f := filtersFromRequest(req)
	where, args, err := buildWhere(f, req.IncludeFixtures)
	if err != nil {
		apiError(w, 400, "invalid_filter", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var total, petCount, intensityCount int
	var petMin, petMax, petMean, intensityMin, intensityMax, intensityMean sql.NullFloat64
	query := "SELECT count(*),count(s.source_pet),min(s.source_pet),max(s.source_pet),avg(s.source_pet),count(s.source_intensity),min(s.source_intensity),max(s.source_intensity),avg(s.source_intensity) FROM scenarios s WHERE " + where
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total, &petCount, &petMin, &petMax, &petMean, &intensityCount, &intensityMin, &intensityMax, &intensityMean); err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	bins := []map[string]any{}
	for _, bin := range []struct {
		label string
		low   float64
		high  *float64
	}{{"below_0_5s", 0, floatValue(.5)}, {"0_5_to_1s", .5, floatValue(1)}, {"1_to_1_5s", 1, floatValue(1.5)}, {"1_5_to_3s", 1.5, floatValue(3)}, {"3s_and_above", 3, nil}} {
		sqlText := "SELECT count(*) FROM scenarios s WHERE " + where + " AND s.source_pet>=?"
		binArgs := append(append([]any{}, args...), bin.low)
		if bin.high != nil {
			sqlText += " AND s.source_pet<?"
			binArgs = append(binArgs, *bin.high)
		}
		var count int
		if err := s.db.QueryRowContext(ctx, sqlText, binArgs...).Scan(&count); err != nil {
			apiError(w, 500, "index_error", err.Error())
			return
		}
		bins = append(bins, map[string]any{"label": bin.label, "lowerInclusive": bin.low, "upperExclusive": bin.high, "count": count})
	}
	respond(w, map[string]any{"populationCount": total, "sourcePET": map[string]any{"count": petCount, "min": nullable(petMin), "max": nullable(petMax), "mean": nullable(petMean), "exploratoryBins": bins}, "sourceInteractionIntensity": map[string]any{"count": intensityCount, "min": nullable(intensityMin), "max": nullable(intensityMax), "mean": nullable(intensityMean)}, "fixturesIncluded": req.IncludeFixtures, "limitations": "PET bins are exploratory; PET is not a collision probability."})
}
func floatValue(v float64) *float64 { return &v }
func (s *service) compareScenarios(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ScenarioIDs     []string `json:"scenarioIds"`
		IncludeFixtures bool     `json:"includeFixtures"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		apiError(w, 400, "invalid_request", err.Error())
		return
	}
	if err := ensureEOF(dec); err != nil {
		apiError(w, 400, "invalid_request", err.Error())
		return
	}
	if len(req.ScenarioIDs) < 2 || len(req.ScenarioIDs) > 4 {
		apiError(w, 400, "invalid_request", "comparison requires between two and four scenario IDs")
		return
	}
	seen := map[string]bool{}
	for _, id := range req.ScenarioIDs {
		if id == "" || seen[id] {
			apiError(w, 400, "invalid_request", "scenario IDs must be nonempty and distinct")
			return
		}
		seen[id] = true
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(req.ScenarioIDs)), ",")
	class := "data_class='research'"
	if req.IncludeFixtures {
		class = "data_class IN ('research','synthetic-fixture')"
	}
	args := make([]any, len(req.ScenarioIDs))
	for i, id := range req.ScenarioIDs {
		args[i] = id
	}
	rows, err := s.db.QueryContext(r.Context(), "SELECT id,data_class,source_dataset,source_release,original_scenario_ref,original_scene_id,original_data_file,source_cache_folder,source_scenario_index,track_ids_json,original_track_ids_json,interaction_start,interaction_end,source_pet,source_intensity,agent_count,agent_count_label,key_agent_ids_json,av_involvement,source_vehicle_types_json,source_path_category,source_path_relation,trajectories_available,map_available,quality_flags_json,original_annotations_json FROM scenarios WHERE "+class+" AND id IN ("+placeholders+")", args...)
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	defer rows.Close()
	items := []Scenario{}
	for rows.Next() {
		scenario, err := scanScenario(rows)
		if err != nil {
			apiError(w, 500, "index_error", err.Error())
			return
		}
		items = append(items, scenario)
	}
	if err := rows.Err(); err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	if len(items) != len(req.ScenarioIDs) {
		apiError(w, 404, "scenario_not_found", "one or more scenarios are unavailable in the requested data class")
		return
	}
	respond(w, map[string]any{"items": items, "timeWarping": false, "note": "Recorded timestamps and sampling intervals are preserved."})
}

func defaultLimit(v int) int {
	if v == 0 {
		return 50
	}
	return v
}
func parsePage(rawLimit, cursor, sortField, direction string, include bool) (page, error) {
	limit := 50
	if rawLimit != "" {
		n, e := strconv.Atoi(rawLimit)
		if e != nil || n < 1 || n > 100 {
			return page{}, errors.New("limit must be between 1 and 100")
		}
		limit = n
	}
	offset := 0
	if cursor != "" {
		decoded, e := base64.RawURLEncoding.DecodeString(cursor)
		if e != nil {
			return page{}, errors.New("invalid cursor")
		}
		offset, e = strconv.Atoi(string(decoded))
		if e != nil || offset < 0 || offset > 1_000_000 {
			return page{}, errors.New("invalid cursor")
		}
	}
	fields := map[string]string{"id": "s.id", "sourcePet": "s.source_pet", "agentCount": "s.agent_count", "dataset": "s.source_dataset"}
	if sortField == "" {
		sortField = "id"
	}
	column, ok := fields[sortField]
	if !ok {
		return page{}, errors.New("unsupported sort field")
	}
	if direction == "" {
		direction = "asc"
	}
	direction = strings.ToLower(direction)
	if direction != "asc" && direction != "desc" {
		return page{}, errors.New("sort direction must be asc or desc")
	}
	return page{limit: limit, offset: offset, sortField: column, sortDirection: direction, includeFixtures: include}, nil
}
func (s *service) runSearch(w http.ResponseWriter, r *http.Request, filters queryFilters, pg page) {
	where, args, err := buildWhere(filters, pg.includeFixtures)
	if err != nil {
		apiError(w, 400, "invalid_filter", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	defer tx.Rollback()
	var total int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM scenarios s WHERE "+where, args...).Scan(&total); err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	researchCount, fixtureCount := total, 0
	if pg.includeFixtures {
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM scenarios s WHERE "+where+" AND s.data_class='research'", args...).Scan(&researchCount); err != nil {
			apiError(w, 500, "index_error", err.Error())
			return
		}
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM scenarios s WHERE "+where+" AND s.data_class='synthetic-fixture'", args...).Scan(&fixtureCount); err != nil {
			apiError(w, 500, "index_error", err.Error())
			return
		}
	}
	query := "SELECT s.id,s.data_class,s.source_dataset,s.source_release,s.original_scenario_ref,s.original_scene_id,s.original_data_file,s.source_cache_folder,s.source_scenario_index,s.track_ids_json,s.original_track_ids_json,s.interaction_start,s.interaction_end,s.source_pet,s.source_intensity,s.agent_count,s.agent_count_label,s.key_agent_ids_json,s.av_involvement,s.source_vehicle_types_json,s.source_path_category,s.source_path_relation,s.trajectories_available,s.map_available,s.quality_flags_json,s.original_annotations_json FROM scenarios s WHERE " + where + " ORDER BY " + pg.sortField + " " + pg.sortDirection + ",s.id ASC LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), pg.limit+1, pg.offset)
	rows, err := tx.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	items := []Scenario{}
	for rows.Next() {
		record, err := scanScenario(rows)
		if err != nil {
			rows.Close()
			apiError(w, 500, "index_error", err.Error())
			return
		}
		items = append(items, record)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		apiError(w, 500, "index_error", rowsErr.Error())
		return
	}
	next := any(nil)
	if len(items) > pg.limit {
		items = items[:pg.limit]
		next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(pg.offset + pg.limit)))
	}
	if err := tx.Commit(); err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	respond(w, map[string]any{"items": items, "nextCursor": next, "total": total, "counts": map[string]int{"research": researchCount, "syntheticFixtures": fixtureCount}, "dataClass": map[bool]string{true: "research-and-opted-in-fixtures", false: "research"}[pg.includeFixtures]})
}
func buildWhere(f queryFilters, include bool) (string, []any, error) {
	parts := []string{}
	args := []any{}
	if include {
		parts = append(parts, "s.data_class IN ('research','synthetic-fixture')")
	} else {
		parts = append(parts, "s.data_class='research'")
	}
	add := func(sql string, v any) { parts = append(parts, sql); args = append(args, v) }
	if f.dataset != "" {
		add("s.source_dataset=?", f.dataset)
	}
	if f.search != "" {
		pattern := "%" + strings.ToLower(f.search) + "%"
		parts = append(parts, "(lower(s.id) LIKE ? OR lower(s.source_dataset) LIKE ? OR lower(coalesce(s.original_scenario_ref,'')) LIKE ? OR lower(coalesce(s.source_path_relation,'')) LIKE ?)")
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if f.relationship != "" {
		add("s.source_path_relation=?", f.relationship)
	}
	if f.pathCategory != "" {
		add("s.source_path_category=?", f.pathCategory)
	}
	if f.avInvolvement != "" {
		add("s.av_involvement=?", f.avInvolvement)
	}
	if f.minAgents != nil {
		add("s.agent_count>=?", *f.minAgents)
	}
	if f.maxAgents != nil {
		add("s.agent_count<=?", *f.maxAgents)
	}
	if f.petValue != nil {
		if !finite(*f.petValue) {
			return "", nil, errors.New("PET threshold must be finite")
		}
		if f.petMetric != "source_pet" {
			return "", nil, errors.New("only source_pet is indexed; derived PET metrics are not available in this release")
		}
		ops := map[string]string{"lt": "<", "lte": "<=", "gt": ">", "gte": ">=", "eq": "="}
		op, ok := ops[f.petOperator]
		if !ok {
			return "", nil, errors.New("PET operator must be lt, lte, gt, gte, or eq")
		}
		add("s.source_pet "+op+" ?", *f.petValue)
	} else if f.petMetric != "" || f.petOperator != "" {
		return "", nil, errors.New("PET metric, operator, and value must be provided together")
	}
	if f.pairScope != "" {
		if f.pairScope != "source_key_pair" {
			return "", nil, errors.New("interaction pair scope must be source_key_pair")
		}
		if len(f.pairAgents) != 2 {
			return "", nil, errors.New("source_key_pair searches require exactly two agent filters")
		}
		pairSQL, pairArgs := pairPredicate(f.pairAgents, f.distinctAgents)
		parts = append(parts, "EXISTS (SELECT 1 FROM interaction_pairs p WHERE p.scenario_id=s.id AND p.source_key_pair=1 AND "+pairSQL+")")
		args = append(args, pairArgs...)
	} else if len(f.pairAgents) > 0 {
		return "", nil, errors.New("agent filters require interactionPair.scope=source_key_pair")
	}
	return strings.Join(parts, " AND "), args, nil
}
func pairPredicate(agents []pairAgentFilter, distinct bool) (string, []any) {
	left, argsLeft := agentPredicate("p.first_agent_id", "p.first_turn", "p.priority_agent_id", agents[0])
	right, argsRight := agentPredicate("p.second_agent_id", "p.second_turn", "p.priority_agent_id", agents[1])
	reverseLeft, argsReverseLeft := agentPredicate("p.first_agent_id", "p.first_turn", "p.priority_agent_id", agents[1])
	reverseRight, argsReverseRight := agentPredicate("p.second_agent_id", "p.second_turn", "p.priority_agent_id", agents[0])
	distinctSQL := ""
	if distinct {
		distinctSQL = " AND p.first_agent_id<>p.second_agent_id"
	}
	return "((" + left + " AND " + right + ") OR (" + reverseLeft + " AND " + reverseRight + "))" + distinctSQL, append(append(append(argsLeft, argsRight...), argsReverseLeft...), argsReverseRight...)
}
func agentPredicate(idColumn, turnColumn, priorityColumn string, filter pairAgentFilter) (string, []any) {
	parts := []string{}
	args := []any{}
	if filter.TurningDirection != "" {
		parts = append(parts, turnColumn+"=?")
		args = append(args, filter.TurningDirection)
	}
	if filter.Priority != nil {
		if *filter.Priority {
			parts = append(parts, priorityColumn+"="+idColumn)
		} else {
			parts = append(parts, priorityColumn+" IS NOT NULL AND "+priorityColumn+"<>"+idColumn)
		}
	}
	if len(parts) == 0 {
		return "1=1", args
	}
	return strings.Join(parts, " AND "), args
}
func scanScenario(rows interface{ Scan(...any) error }) (Scenario, error) {
	var x Scenario
	var dataset, class, release string
	var trackIDs, origTrackIDs, keyIDs, quality, annotations string
	var vehicleTypes sql.NullString
	var traj, maps int
	var start, end, pet, intensity sql.NullFloat64
	var ref, scene, file, folder, idx, agentLabel, av, category, relation sql.NullString
	err := rows.Scan(&x.ID, &class, &dataset, &release, &ref, &scene, &file, &folder, &idx, &trackIDs, &origTrackIDs, &start, &end, &pet, &intensity, &x.AgentCount, &agentLabel, &keyIDs, &av, &vehicleTypes, &category, &relation, &traj, &maps, &quality, &annotations)
	if err != nil {
		return x, err
	}
	x.DataClass = class
	x.SourceDataset = dataset
	x.SourceRelease = release
	x.OriginalScenarioRef = stringPtr(ref)
	x.OriginalSceneID = stringPtr(scene)
	x.OriginalDataFile = stringPtr(file)
	x.SourceCacheFolder = stringPtr(folder)
	x.SourceScenarioIndex = stringPtr(idx)
	x.InteractionStart = floatPtr(start)
	x.InteractionEnd = floatPtr(end)
	x.SourcePET = floatPtr(pet)
	x.SourceIntensity = floatPtr(intensity)
	x.AgentCountLabel = stringPtr(agentLabel)
	x.AVInvolvement = stringPtr(av)
	x.SourcePathCategory = stringPtr(category)
	x.SourcePathRelation = stringPtr(relation)
	x.TrajectoriesAvailable = traj == 1
	x.MapAvailable = maps == 1
	_ = json.Unmarshal([]byte(trackIDs), &x.TrackIDs)
	_ = json.Unmarshal([]byte(origTrackIDs), &x.OriginalTrackIDs)
	_ = json.Unmarshal([]byte(keyIDs), &x.KeyAgentIDs)
	if vehicleTypes.Valid {
		_ = json.Unmarshal([]byte(vehicleTypes.String), &x.SourceVehicleTypes)
	}
	_ = json.Unmarshal([]byte(quality), &x.QualityFlags)
	_ = json.Unmarshal([]byte(annotations), &x.OriginalAnnotations)
	if x.DataClass == "synthetic-fixture" {
		var fixture Scenario
		if json.Unmarshal([]byte(annotations), &fixture) == nil {
			fixture.DataClass = x.DataClass
			return fixture, nil
		}
	}
	return x, nil
}
func stringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}
func floatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	n := v.Float64
	return &n
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func (s *service) getScenario(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	include := r.URL.Query().Get("includeFixtures") == "true"
	where := "id=? AND data_class='research'"
	if include {
		where = "id=? AND data_class IN ('research','synthetic-fixture')"
	}
	row := s.db.QueryRowContext(r.Context(), "SELECT id,data_class,source_dataset,source_release,original_scenario_ref,original_scene_id,original_data_file,source_cache_folder,source_scenario_index,track_ids_json,original_track_ids_json,interaction_start,interaction_end,source_pet,source_intensity,agent_count,agent_count_label,key_agent_ids_json,av_involvement,source_vehicle_types_json,source_path_category,source_path_relation,trajectories_available,map_available,quality_flags_json,original_annotations_json FROM scenarios WHERE "+where, id)
	scenario, err := scanScenario(row)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	respond(w, scenario)
}
func (s *service) scenarioAgents(w http.ResponseWriter, r *http.Request) {
	scenario, ok := s.loadScenario(w, r)
	if !ok {
		return
	}
	if scenario.DataClass == "synthetic-fixture" {
		respond(w, scenario.Agents)
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT agent_id,object_class,automation_class,length_m,width_m,height_m,dimension_provenance,is_source_key_agent,source_turning_direction,source_priority_status,first_valid_time,last_valid_time FROM agents WHERE scenario_id=? ORDER BY agent_id`, scenario.ID)
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	defer rows.Close()
	agents := []map[string]any{}
	for rows.Next() {
		var id string
		var objectClass, automationClass, provenance, turn sql.NullString
		var length, width, height, first, last sql.NullFloat64
		var isKey int
		var priority sql.NullInt64
		if err = rows.Scan(&id, &objectClass, &automationClass, &length, &width, &height, &provenance, &isKey, &turn, &priority, &first, &last); err != nil {
			apiError(w, 500, "index_error", err.Error())
			return
		}
		dimensions := map[string]any{"lengthM": nullable(length), "widthM": nullable(width), "heightM": nullable(height), "provenance": nullString(provenance)}
		var priorityValue any
		if priority.Valid {
			priorityValue = priority.Int64 == 1
		}
		agents = append(agents, map[string]any{"id": id, "scenarioId": scenario.ID, "objectClass": nullString(objectClass), "automationClass": nullString(automationClass), "dimensions": dimensions, "sourceKeyAgent": isKey == 1, "turningDirection": nullString(turn), "priority": priorityValue, "firstValidTime": nullable(first), "lastValidTime": nullable(last), "trajectoryAvailable": scenario.TrajectoriesAvailable})
	}
	if err = rows.Err(); err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	respond(w, agents)
}
func nullString(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	return v.String
}
func (s *service) scenarioTrajectories(w http.ResponseWriter, r *http.Request) {
	scenario, ok := s.loadScenario(w, r)
	if !ok {
		return
	}
	if scenario.DataClass == "synthetic-fixture" && len(scenario.Agents) > 0 {
		respond(w, map[string]any{"scenarioId": scenario.ID, "available": true, "coordinateFrame": "local-metric", "agents": scenario.Agents})
		return
	}
	if !scenario.TrajectoriesAvailable {
		respond(w, map[string]any{"scenarioId": scenario.ID, "available": false, "reason": "trajectory samples were not imported with the metadata index"})
		return
	}
	respond(w, map[string]any{"scenarioId": scenario.ID, "available": false, "reason": "trajectory asset handler is not configured"})
}
func (s *service) scenarioMap(w http.ResponseWriter, r *http.Request) {
	scenario, ok := s.loadScenario(w, r)
	if !ok {
		return
	}
	respond(w, map[string]any{"scenarioId": scenario.ID, "available": scenario.MapAvailable, "features": []any{}})
}
func (s *service) scenarioMetrics(w http.ResponseWriter, r *http.Request) {
	scenario, ok := s.loadScenario(w, r)
	if !ok {
		return
	}
	respond(w, map[string]any{"scenarioId": scenario.ID, "sourcePet": scenario.SourcePET, "sourceIntensity": scenario.SourceIntensity, "derivedMetrics": map[string]any{"centerPointPet": nil, "vehicleFootprintPet": nil, "reason": "trajectory asset unavailable or metric not computed"}})
}
func (s *service) scenarioConflicts(w http.ResponseWriter, r *http.Request) {
	scenario, ok := s.loadScenario(w, r)
	if !ok {
		return
	}
	respond(w, map[string]any{"scenarioId": scenario.ID, "events": []any{}, "available": false})
}
func (s *service) scenarioQuality(w http.ResponseWriter, r *http.Request) {
	scenario, ok := s.loadScenario(w, r)
	if !ok {
		return
	}
	findings := []map[string]any{}
	for _, flag := range scenario.QualityFlags {
		findings = append(findings, map[string]any{"code": flag, "severity": "warning", "provenance": "metadata normalization"})
	}
	respond(w, map[string]any{"scenarioId": scenario.ID, "findings": findings})
}
func (s *service) loadScenario(w http.ResponseWriter, r *http.Request) (Scenario, bool) {
	query := "SELECT id,data_class,source_dataset,source_release,original_scenario_ref,original_scene_id,original_data_file,source_cache_folder,source_scenario_index,track_ids_json,original_track_ids_json,interaction_start,interaction_end,source_pet,source_intensity,agent_count,agent_count_label,key_agent_ids_json,av_involvement,source_vehicle_types_json,source_path_category,source_path_relation,trajectories_available,map_available,quality_flags_json,original_annotations_json FROM scenarios WHERE id=?"
	rows, err := s.db.QueryContext(r.Context(), query, r.PathValue("id"))
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return Scenario{}, false
	}
	defer rows.Close()
	if !rows.Next() {
		http.NotFound(w, r)
		return Scenario{}, false
	}
	scenario, err := scanScenario(rows)
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return Scenario{}, false
	}
	if scenario.DataClass == "synthetic-fixture" && r.URL.Query().Get("includeFixtures") != "true" {
		http.NotFound(w, r)
		return Scenario{}, false
	}
	return scenario, true
}
func (s *service) relationships(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), "SELECT source_path_category,source_path_relation,count(*) FROM scenarios WHERE data_class='research' GROUP BY source_path_category,source_path_relation ORDER BY 1,2")
	if err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var category, relation sql.NullString
		var count int
		if err := rows.Scan(&category, &relation, &count); err != nil {
			apiError(w, 500, "index_error", err.Error())
			return
		}
		items = append(items, map[string]any{"pathCategory": nullableString(category), "pathRelation": nullableString(relation), "count": count})
	}
	respond(w, items)
}
func nullableString(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	return v.String
}
func (s *service) quality(w http.ResponseWriter, r *http.Request) {
	var records, errorsCount, warnings int
	if err := s.db.QueryRowContext(r.Context(), `SELECT count(*),sum(CASE WHEN q.severity='error' THEN 1 ELSE 0 END),sum(CASE WHEN q.severity='warning' THEN 1 ELSE 0 END) FROM scenarios s LEFT JOIN quality_findings q ON q.scenario_id=s.id WHERE s.data_class='research'`).Scan(&records, &errorsCount, &warnings); err != nil {
		apiError(w, 500, "index_error", err.Error())
		return
	}
	respond(w, map[string]any{"researchRecords": records, "errors": errorsCount, "warnings": warnings})
}
func (s *service) openapi(w http.ResponseWriter, r *http.Request) {
	paths := map[string]any{}
	for _, p := range []string{"/v1/health", "/v1/releases", "/v1/overview", "/v1/datasets", "/v1/datasets/{id}", "/v1/datasets/{id}/statistics", "/v1/scenarios", "/v1/scenarios/search", "/v1/scenarios/compare", "/v1/analytics/query", "/v1/scenarios/{id}", "/v1/scenarios/{id}/agents", "/v1/scenarios/{id}/trajectories", "/v1/scenarios/{id}/map", "/v1/scenarios/{id}/metrics", "/v1/scenarios/{id}/conflicts", "/v1/scenarios/{id}/quality", "/v1/relationships", "/v1/quality"} {
		method := "get"
		if p == "/v1/scenarios/search" {
			method = "post"
		}
		paths[p] = map[string]any{method: map[string]string{"summary": "Traffic Conflict Atlas " + method + " " + p}}
	}
	respond(w, map[string]any{"openapi": "3.1.0", "info": map[string]string{"title": "Traffic Conflict Atlas API", "version": "0.2.0"}, "paths": paths})
}
func ensureEOF(d *json.Decoder) error {
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
func apiError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("method=%s path=%s elapsed=%s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
