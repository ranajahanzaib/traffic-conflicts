import unittest
from pipelines.traffic_conflicts.normalization import normalize_interhub_row

ROW={"dataset":"interaction_multi","folder":"cache/release","scenario_idx":"12","track_id":"a;b;c","start":"1.25","end":"6.25","intensity":"2.4","PET":"0.73","two/multi":"multi","vehicle_type":"['HV', 'AV']","AV_included":"AV","key_agents":"a;b","path_category":"CP","path_relation":"C-O","turn_label":"L-S","priority_label":"b","original_data_file":"scene.csv","original_scene_id":"scene-01","original_track_id":"101;202;303"}
class InterHubNormalizationTests(unittest.TestCase):
    def test_preserves_source_annotations_and_key_pair_semantics(self):
        record=normalize_interhub_row(ROW,"figshare-r1")
        self.assertEqual(record.source_release,"figshare-r1")
        self.assertEqual(record.key_agent_ids,("a","b"))
        self.assertEqual(record.key_pair_turns,("L","S"))
        self.assertEqual(record.priority_agent_id,"b")
        self.assertEqual(record.agent_count,3)
        self.assertEqual(record.source_pet,.73)
        self.assertFalse(record.trajectories_available)
        self.assertFalse(record.map_available)
        self.assertEqual(record.original_annotations["PET"],"0.73")
    def test_deterministic_id(self):
        self.assertEqual(normalize_interhub_row(ROW,"r1").scenario_id,normalize_interhub_row(dict(ROW),"r2").scenario_id)
    def test_flags_invalid_priority_reference(self):
        row={**ROW,"priority_label":"stranger"}
        self.assertIn("priority_agent_not_in_key_pair",normalize_interhub_row(row,"r1").quality_flags)
    def test_missing_measurement_stays_null(self):
        row={**ROW,"PET":"","intensity":"nan"}
        record=normalize_interhub_row(row,"r1")
        self.assertIsNone(record.source_pet)
        self.assertIsNone(record.source_intensity)
        self.assertIn("nonfinite_source_intensity",record.quality_flags)
if __name__=="__main__": unittest.main()
