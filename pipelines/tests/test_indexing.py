import csv,sqlite3,tempfile,unittest
from pathlib import Path
from pipelines.traffic_conflicts.adapters.interhub import REQUIRED_COLUMNS
from pipelines.traffic_conflicts.indexing import build_index,build_demo_index
from pipelines.traffic_conflicts.aggregates import build_aggregates
from test_interhub_adapter import ROW
LICENSE={"dataset":"interaction_multi","review_status":"reviewed","dataset_license":"unit test source terms","attribution":"test dataset attribution","redistribution_allowed":False,"map_redistribution_allowed":False,"derived_data_allowed":False,"commercial_use_allowed":False,"evidence_url":"https://example.org/license","reviewed_at":"2026-11-26"}
class IndexTests(unittest.TestCase):
    def make_csv(self,path,rows):
        with path.open('w',newline='',encoding='utf-8') as f:
            writer=csv.DictWriter(f,fieldnames=sorted(REQUIRED_COLUMNS));writer.writeheader();writer.writerows(rows)
    def test_builds_index_and_pair_indexes_transactionally(self):
        with tempfile.TemporaryDirectory() as tmp:
            source=Path(tmp)/'source.csv';out=Path(tmp)/'index.sqlite';self.make_csv(source,[ROW])
            report=build_index(source,out,'release-test',source_dataset='interaction_multi',license_manifest=LICENSE)
            self.assertEqual(report['imported'],1);self.assertEqual(report['quarantined'],0)
            db=sqlite3.connect(out)
            self.assertEqual(db.execute('select count(*) from scenarios where data_class="research"').fetchone()[0],1)
            pair=db.execute('select first_agent_id,second_agent_id,first_turn,second_turn,priority_agent_id from interaction_pairs').fetchone()
            self.assertEqual(pair,('a','b','L','S','b'))
            agent_rows=db.execute('select agent_id,is_source_key_agent,source_turning_direction,source_priority_status from agents order by agent_id').fetchall()
            self.assertEqual(agent_rows,[('a',1,'L',0),('b',1,'S',1),('c',0,None,None)])
            rights=db.execute('select dataset_license,redistribution_allowed,map_redistribution_allowed,derived_data_allowed,commercial_use_allowed from source_licenses').fetchone()
            self.assertEqual(rights,('unit test source terms',0,0,0,0))
            self.assertEqual(db.execute('select trajectories_available,map_available from scenarios').fetchone(),(0,0))
            self.assertTrue(db.execute("select name from sqlite_master where type='index' and name='idx_scenarios_pet'").fetchone())
    def test_invalid_rows_are_quarantined_not_silently_dropped(self):
        with tempfile.TemporaryDirectory() as tmp:
            source=Path(tmp)/'source.csv';out=Path(tmp)/'index.sqlite';self.make_csv(source,[{**ROW,'start':'9','end':'3'}])
            report=build_index(source,out,'release-test',source_dataset='interaction_multi',license_manifest=LICENSE)
            self.assertEqual(report['imported'],0);self.assertEqual(report['quarantined'],1)
            self.assertTrue(Path(report['quarantineFile']).exists())
    def test_duplicate_rows_quarantined(self):
        with tempfile.TemporaryDirectory() as tmp:
            source=Path(tmp)/'source.csv';out=Path(tmp)/'index.sqlite';self.make_csv(source,[ROW,ROW])
            report=build_index(source,out,'release-test',source_dataset='interaction_multi',license_manifest=LICENSE)
            self.assertEqual(report['imported'],1);self.assertEqual(report['quarantined'],1)
    def test_demo_index_keeps_fixture_out_of_research_count(self):
        with tempfile.TemporaryDirectory() as tmp:
            out=Path(tmp)/'demo.sqlite';fixture=Path('data/fixtures/synthetic-crossing.json')
            build_demo_index(out,fixture);db=sqlite3.connect(out)
            self.assertEqual(db.execute("select count(*) from scenarios where data_class='research'").fetchone()[0],0)
            self.assertEqual(db.execute("select count(*) from scenarios where data_class='synthetic-fixture'").fetchone()[0],1)
            self.assertEqual(db.execute("select count(*) from agents where scenario_id='fixture-crossing-001'").fetchone()[0],2)
            self.assertEqual(db.execute("select count(*) from trajectory_assets where scenario_id='fixture-crossing-001'").fetchone()[0],2)
    def test_release_aggregates_use_research_rows_and_exact_bins(self):
        with tempfile.TemporaryDirectory() as tmp:
            source=Path(tmp)/'source.csv';dbpath=Path(tmp)/'index.sqlite';out=Path(tmp)/'aggregate.json';self.make_csv(source,[ROW])
            build_index(source,dbpath,'release-test',source_dataset='interaction_multi',license_manifest=LICENSE)
            result=build_aggregates(dbpath,out)
            self.assertEqual(result['researchScenarioCount'],1)
            self.assertTrue(result['fixtureRowsExcluded'])
            self.assertEqual(result['datasets'][0]['sourcePET']['count'],1)
            self.assertEqual(sum(b['count'] for b in result['datasets'][0]['sourcePET']['exploratoryBins']),1)
    def test_license_manifest_requires_review_and_rights_fields(self):
        with tempfile.TemporaryDirectory() as tmp:
            source=Path(tmp)/'source.csv';out=Path(tmp)/'index.sqlite';self.make_csv(source,[ROW])
            with self.assertRaises(ValueError): build_index(source,out,'release-test',source_dataset='interaction_multi',license_manifest={**LICENSE,'review_status':'unreviewed'})
if __name__=='__main__':unittest.main()
