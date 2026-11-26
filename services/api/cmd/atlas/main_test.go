package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func testService(t *testing.T) *service {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	schema, err := os.ReadFile(filepath.Join("..", "..", "internal", "index", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO releases VALUES('r1','test','now',2,1,'{}'),('fixture-r1','fixture','now',1,0,'{}')")
	if err != nil {
		t.Fatal(err)
	}
	insert := func(id, class, dataset, release string, pet any, count int, agents string, relation string, av string) {
		t.Helper()
		_, e := db.Exec(`INSERT INTO scenarios(id,data_class,source_dataset,source_release,track_ids_json,original_track_ids_json,agent_count,key_agent_ids_json,av_involvement,source_path_relation,trajectories_available,map_available,quality_flags_json,original_annotations_json,source_pet) VALUES(?,?,?,?,?,?,?,?,?,?,0,0,'[]','{}',?)`, id, class, dataset, release, agents, "[]", count, "[]", av, relation, pet)
		if e != nil {
			t.Fatal(e)
		}
	}
	insert("research-1", "research", "waymo_train", "r1", 0.7, 3, `["a","b","c"]`, "C-O", "AV")
	insert("research-2", "research", "waymo_train", "r1", 1.5, 2, `["c","d"]`, "C-O", "all_HV")
	insert("fixture-crossing-001", "synthetic-fixture", "synthetic-fixture", "fixture-r1", nil, 2, `["vehicle-a","vehicle-b"]`, "crossing", "")
	fixturePayload := `{"id":"fixture-crossing-001","sourceDataset":"synthetic-fixture","sourceRelease":"fixture-r1","provenance":"synthetic fixture","agentCount":2,"agents":[{"id":"vehicle-a","samples":[{"t":0,"x":-1,"y":0}]},{"id":"vehicle-b","samples":[{"t":0,"x":0,"y":-1}]}]}`
	if _, err := db.Exec("UPDATE scenarios SET original_annotations_json=? WHERE id='fixture-crossing-001'", fixturePayload); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO interaction_pairs VALUES('research-1','a','b',1,'L','S','a'),('research-2','c','d',1,'S','R','d')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO agents(scenario_id,agent_id,source_agent_id,is_source_key_agent,source_turning_direction,source_priority_status) VALUES('research-1','a','a',1,'L',1),('research-1','b','b',1,'S',0),('research-1','c','c',0,NULL,NULL)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &service{db: db, webRoot: "../../../../apps/web/dist"}
}
func request(t *testing.T, s *service, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)
	return rec
}
func TestResearchSearchExcludesFixturesByDefault(t *testing.T) {
	s := testService(t)
	rec := request(t, s, "GET", "/v1/scenarios", "")
	var got struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 {
		t.Fatalf("total=%d, want 2", got.Total)
	}
}
func TestFixtureSearchNeedsExplicitOptIn(t *testing.T) {
	s := testService(t)
	rec := request(t, s, "GET", "/v1/scenarios?includeFixtures=true", "")
	var got struct {
		Total     int    `json:"total"`
		DataClass string `json:"dataClass"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 3 || got.DataClass != "research-and-opted-in-fixtures" {
		t.Fatalf("response=%+v", got)
	}
	rec = request(t, s, "GET", "/v1/scenarios/fixture-crossing-001", "")
	if rec.Code != 404 {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
}
func TestPETThresholdUsesFilteredPopulation(t *testing.T) {
	s := testService(t)
	rec := request(t, s, "GET", "/v1/scenarios?sourcePetLt=1", "")
	var got struct {
		Total int        `json:"total"`
		Items []Scenario `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || got.Items[0].ID != "research-1" {
		t.Fatalf("response=%+v", got)
	}
}
func TestPairSearchMatchesDistinctAgentsInsideSameKeyPair(t *testing.T) {
	s := testService(t)
	body := `{"filters":{"interactionPair":{"scope":"source_key_pair","distinctAgents":true,"agents":[{"turningDirection":"L","priority":true},{"turningDirection":"S","priority":false}]}},"limit":20}`
	rec := request(t, s, "POST", "/v1/scenarios/search", body)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Total int        `json:"total"`
		Items []Scenario `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || got.Items[0].ID != "research-1" {
		t.Fatalf("response=%+v", got)
	}
}
func TestSearchRejectsUnsupportedPETMetric(t *testing.T) {
	s := testService(t)
	body := `{"filters":{"pet":{"metric":"derived_pet","operator":"lt","value":1}}}`
	rec := request(t, s, "POST", "/v1/scenarios/search", body)
	if rec.Code != 400 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
func TestParameterizedDatasetSearchAndPagination(t *testing.T) {
	s := testService(t)
	rec := request(t, s, "GET", "/v1/scenarios?dataset=%27%20OR%201%3D1%20--&limit=1", "")
	var got struct {
		Total int        `json:"total"`
		Items []Scenario `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 0 {
		t.Fatalf("injection-like value returned %d records", got.Total)
	}
	rec = request(t, s, "GET", "/v1/scenarios?limit=1", "")
	var pageOne struct {
		Total int        `json:"total"`
		Items []Scenario `json:"items"`
		Next  *string    `json:"nextCursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pageOne); err != nil {
		t.Fatal(err)
	}
	if len(pageOne.Items) != 1 || pageOne.Next == nil {
		t.Fatalf("page=%+v", pageOne)
	}
	rec = request(t, s, "GET", "/v1/scenarios?limit=1&cursor="+*pageOne.Next, "")
	var pageTwo struct {
		Items []Scenario `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &pageTwo)
	if len(pageTwo.Items) != 1 || pageOne.Items[0].ID == pageTwo.Items[0].ID {
		t.Fatalf("pagination repeated or omitted item: %v %v", pageOne.Items, pageTwo.Items)
	}
}
func TestOpenAPIAndHealth(t *testing.T) {
	s := testService(t)
	if rec := request(t, s, "GET", "/v1/openapi.json", ""); rec.Code != 200 {
		t.Fatalf("openapi status=%d", rec.Code)
	}
	rec := request(t, s, "GET", "/v1/health", "")
	if rec.Code != 200 {
		t.Fatalf("health status=%d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("health=%v", body)
	}
}
func TestQueryRespectsRequestCancellation(t *testing.T) {
	s := testService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest("GET", "/v1/scenarios", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)
	if rec.Code < 400 {
		t.Fatalf("canceled request status=%d", rec.Code)
	}
}

func TestFixtureTrajectoryEndpointReturnsFixtureSamplesAfterOptIn(t *testing.T) {
	s := testService(t)
	rec := request(t, s, "GET", "/v1/scenarios/fixture-crossing-001/trajectories?includeFixtures=true", "")
	var got struct {
		Available bool    `json:"available"`
		Agents    []Agent `json:"agents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Available || len(got.Agents) != 2 {
		t.Fatalf("response=%+v", got)
	}
}
func TestAnalyticsUsesExactFilteredPopulation(t *testing.T) {
	s := testService(t)
	body := `{"filters":{"pet":{"metric":"source_pet","operator":"lt","value":1}}}`
	rec := request(t, s, "POST", "/v1/analytics/query", body)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Population int `json:"populationCount"`
		PET        struct {
			Count int `json:"count"`
		} `json:"sourcePET"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Population != 1 || got.PET.Count != 1 {
		t.Fatalf("analytics=%+v", got)
	}
}
func TestScenarioComparisonPreservesRecordedTimes(t *testing.T) {
	s := testService(t)
	rec := request(t, s, "POST", "/v1/scenarios/compare", `{"scenarioIds":["research-1","research-2"]}`)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		TimeWarping bool       `json:"timeWarping"`
		Items       []Scenario `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.TimeWarping || len(got.Items) != 2 {
		t.Fatalf("comparison=%+v", got)
	}
}

func TestScenarioAgentsUsePerAgentIndexedProvenance(t *testing.T) {
	s := testService(t)
	rec := request(t, s, "GET", "/v1/scenarios/research-1/agents", "")
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0]["turningDirection"] != "L" || got[0]["priority"] != true || got[2]["sourceKeyAgent"] != false || got[2]["turningDirection"] != nil || got[2]["priority"] != nil {
		t.Fatalf("agents=%v", got)
	}
}

var _ http.Handler
