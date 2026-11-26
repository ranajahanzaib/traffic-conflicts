PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS releases(
 release_id TEXT PRIMARY KEY, source TEXT NOT NULL, imported_at TEXT NOT NULL,
 scenario_count INTEGER NOT NULL, metadata_only INTEGER NOT NULL, license_json TEXT NOT NULL DEFAULT '{}');
CREATE TABLE IF NOT EXISTS scenarios(
 id TEXT PRIMARY KEY, data_class TEXT NOT NULL CHECK(data_class IN ('research','synthetic-fixture')),
 source_dataset TEXT NOT NULL, source_release TEXT NOT NULL REFERENCES releases(release_id) DEFERRABLE INITIALLY DEFERRED,
 original_scenario_ref TEXT, original_scene_id TEXT, original_data_file TEXT, source_cache_folder TEXT, source_scenario_index TEXT,
 track_ids_json TEXT NOT NULL, original_track_ids_json TEXT NOT NULL,
 interaction_start REAL, interaction_end REAL, source_pet REAL, source_intensity REAL,
 agent_count INTEGER NOT NULL, agent_count_label TEXT, key_agent_ids_json TEXT NOT NULL,
 av_involvement TEXT, source_vehicle_types_json TEXT, source_path_category TEXT, source_path_relation TEXT,
 trajectories_available INTEGER NOT NULL, map_available INTEGER NOT NULL,
 quality_flags_json TEXT NOT NULL, original_annotations_json TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS source_licenses(
 source_dataset TEXT NOT NULL, source_release TEXT NOT NULL REFERENCES releases(release_id) ON DELETE CASCADE,
 code_license TEXT, dataset_license TEXT NOT NULL, attribution TEXT NOT NULL,
 redistribution_allowed INTEGER NOT NULL, map_redistribution_allowed INTEGER NOT NULL,
 derived_data_allowed INTEGER NOT NULL, commercial_use_allowed INTEGER NOT NULL,
 evidence_url TEXT NOT NULL, reviewed_at TEXT NOT NULL,
 PRIMARY KEY(source_dataset,source_release));
CREATE TABLE IF NOT EXISTS agents(
 scenario_id TEXT NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE, agent_id TEXT NOT NULL,
 source_agent_id TEXT NOT NULL, object_class TEXT, automation_class TEXT,
 length_m REAL, width_m REAL, height_m REAL, dimension_provenance TEXT,
 is_source_key_agent INTEGER NOT NULL DEFAULT 0, source_turning_direction TEXT,
 source_priority_status INTEGER, first_valid_time REAL, last_valid_time REAL,
 PRIMARY KEY(scenario_id,agent_id));
CREATE TABLE IF NOT EXISTS interaction_pairs(
 scenario_id TEXT NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
 first_agent_id TEXT NOT NULL, second_agent_id TEXT NOT NULL, source_key_pair INTEGER NOT NULL,
 first_turn TEXT, second_turn TEXT, priority_agent_id TEXT,
 PRIMARY KEY(scenario_id,first_agent_id,second_agent_id));
CREATE TABLE IF NOT EXISTS trajectory_assets(
 scenario_id TEXT NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
 agent_id TEXT, format TEXT NOT NULL, uri TEXT NOT NULL, sha256 TEXT, sample_count INTEGER,
 time_start REAL, time_end REAL, coordinate_frame TEXT, PRIMARY KEY(scenario_id,agent_id));
CREATE TABLE IF NOT EXISTS map_assets(
 scenario_id TEXT NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
 format TEXT NOT NULL, uri TEXT NOT NULL, sha256 TEXT, coordinate_frame TEXT, attribution TEXT,
 PRIMARY KEY(scenario_id,format));
CREATE TABLE IF NOT EXISTS map_features(
 scenario_id TEXT NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
 feature_id TEXT NOT NULL, feature_type TEXT NOT NULL, geometry_json TEXT NOT NULL,
 provenance TEXT NOT NULL, PRIMARY KEY(scenario_id,feature_id));
CREATE TABLE IF NOT EXISTS conflict_events(
 event_id TEXT PRIMARY KEY, scenario_id TEXT NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
 agent_ids_json TEXT NOT NULL, metric_type TEXT NOT NULL, conflict_geometry_json TEXT,
 first_entry REAL, first_exit REAL, second_entry REAL, second_exit REAL,
 pet_seconds REAL, measurement_method TEXT, measurement_resolution REAL,
 quality_flags_json TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS quality_findings(
 scenario_id TEXT NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
 code TEXT NOT NULL, severity TEXT NOT NULL CHECK(severity IN ('warning','error')),
 detail TEXT NOT NULL, provenance TEXT, PRIMARY KEY(scenario_id,code));
CREATE INDEX IF NOT EXISTS idx_scenarios_dataset ON scenarios(source_dataset,data_class);
CREATE INDEX IF NOT EXISTS idx_scenarios_pet ON scenarios(source_pet) WHERE source_pet IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_scenarios_intensity ON scenarios(source_intensity) WHERE source_intensity IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_scenarios_agents ON scenarios(agent_count);
CREATE INDEX IF NOT EXISTS idx_scenarios_av ON scenarios(av_involvement);
CREATE INDEX IF NOT EXISTS idx_scenarios_relation ON scenarios(source_path_relation);
CREATE INDEX IF NOT EXISTS idx_scenarios_turn ON scenarios(source_path_category);
CREATE INDEX IF NOT EXISTS idx_agents_scenario ON agents(scenario_id,is_source_key_agent);
CREATE INDEX IF NOT EXISTS idx_agents_priority ON agents(scenario_id,source_priority_status);
CREATE INDEX IF NOT EXISTS idx_quality_severity ON quality_findings(severity,scenario_id);
CREATE INDEX IF NOT EXISTS idx_pairs_priority ON interaction_pairs(priority_agent_id);
CREATE INDEX IF NOT EXISTS idx_pairs_key_pair ON interaction_pairs(scenario_id,source_key_pair);
