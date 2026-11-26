"""Build a versioned SQLite scenario index from local InterHub metadata CSV."""
from __future__ import annotations
import argparse, json, os, sqlite3
from datetime import datetime,timezone
from pathlib import Path
from pipelines.traffic_conflicts.adapters.interhub import iter_metadata

def schema_sql() -> str:
    schema_path=Path(__file__).resolve().parents[2]/"schemas"/"index.sql"
    return schema_path.read_text(encoding="utf-8")




def build_index(input_path: Path, output_path: Path, source_release: str, limit: int|None=None, resume: bool=False, source_dataset: str|None=None, license_manifest: dict|None=None) -> dict:
    if not source_release.strip(): raise ValueError("source_release is required to preserve provenance")
    if license_manifest is None: raise ValueError("a reviewed per-source license manifest is required")
    if source_dataset and license_manifest.get("dataset") != source_dataset: raise ValueError("license manifest dataset does not match selected source")
    if license_manifest.get("review_status") != "reviewed" or not license_manifest.get("dataset_license") or not license_manifest.get("attribution") or not license_manifest.get("evidence_url") or not license_manifest.get("reviewed_at"):
        raise ValueError("license manifest must be reviewed and include license, attribution, evidence URL, and review date")
    for right in ("redistribution_allowed","map_redistribution_allowed","derived_data_allowed","commercial_use_allowed"):
        if not isinstance(license_manifest.get(right),bool): raise ValueError(f"license manifest must set boolean {right}")
    output_path.parent.mkdir(parents=True,exist_ok=True)
    temp_path=output_path if resume else output_path.with_name(output_path.name+'.tmp')
    if not resume and temp_path.exists(): temp_path.unlink()
    connection=sqlite3.connect(temp_path)
    connection.execute("PRAGMA journal_mode=WAL")
    connection.executescript(schema_sql())
    imported_at=datetime.now(timezone.utc).isoformat()
    ok=0; quarantined=[]; seen=set()
    try:
        with connection:
            for record in iter_metadata(input_path,source_release,None):
                if source_dataset and record.source_dataset != source_dataset: continue
                if limit is not None and ok+len(quarantined)>=limit: break
                values=record.as_dict()
                sid=values['scenario_id']
                if sid in seen:
                    quarantined.append({"scenario_id":sid,"reason":"duplicate_deterministic_id","source":values['original_annotations']});continue
                seen.add(sid)
                if any(flag.startswith('invalid_') or flag=='interaction_end_before_start' for flag in values['quality_flags']):
                    quarantined.append({"scenario_id":sid,"reason":";".join(values['quality_flags']),"source":values['original_annotations']});continue
                columns=("id,data_class,source_dataset,source_release,original_scenario_ref,original_scene_id,original_data_file,source_cache_folder,source_scenario_index,track_ids_json,original_track_ids_json,interaction_start,interaction_end,source_pet,source_intensity,agent_count,agent_count_label,key_agent_ids_json,av_involvement,source_vehicle_types_json,source_path_category,source_path_relation,trajectories_available,map_available,quality_flags_json,original_annotations_json")
                vals=(sid,'research',values['source_dataset'],source_release,values['original_scenario_ref'],values['original_scene_id'],values['original_data_file'],values['source_cache_folder'],values['source_scenario_index'],json.dumps(values['source_track_ids']),json.dumps(values['original_track_ids']),values['interaction_start'],values['interaction_end'],values['source_pet'],values['source_intensity'],values['agent_count'],values['agent_count_label'],json.dumps(values['key_agent_ids']),values['av_involvement'],json.dumps(values['source_vehicle_types']),values['source_path_category'],values['source_path_relation'],0,0,json.dumps(values['quality_flags']),json.dumps(values['original_annotations'],ensure_ascii=False))
                connection.execute(f"INSERT OR REPLACE INTO scenarios({','.join(columns.split(','))}) VALUES({','.join('?' for _ in vals)})",vals)
                pair=values['key_agent_ids']
                turns=values['key_pair_turns']
                if len(pair)==2 and pair[0]!=pair[1]:
                    connection.execute("INSERT OR REPLACE INTO interaction_pairs(scenario_id,first_agent_id,second_agent_id,source_key_pair,first_turn,second_turn,priority_agent_id) VALUES(?,?,?,?,?,?,?)",(sid,pair[0],pair[1],1,turns[0] if len(turns)==2 else None,turns[1] if len(turns)==2 else None,values['priority_agent_id']))
                distinct_tracks=tuple(dict.fromkeys(values['source_track_ids']))
                for agent_id in distinct_tracks:
                    is_key=agent_id in pair
                    turn=turns[pair.index(agent_id)] if is_key and len(turns)==2 and pair.index(agent_id)<2 else None
                    priority_status=(1 if values['priority_agent_id']==agent_id else 0) if is_key and values['priority_agent_id'] in pair else None
                    connection.execute("INSERT OR REPLACE INTO agents(scenario_id,agent_id,source_agent_id,is_source_key_agent,source_turning_direction,source_priority_status) VALUES(?,?,?,?,?,?)",(sid,agent_id,agent_id,int(is_key),turn,priority_status))
                for flag in values['quality_flags']:
                    connection.execute("INSERT OR REPLACE INTO quality_findings(scenario_id,code,severity,detail,provenance) VALUES(?,?,?,?,?)",(sid,flag,'warning',flag.replace('_',' '),'InterHub metadata normalization'))
                ok+=1
            if resume:
                ok=connection.execute("SELECT count(*) FROM scenarios WHERE source_release=? AND data_class='research'",(source_release,)).fetchone()[0]
            connection.execute("INSERT OR REPLACE INTO releases VALUES(?,?,?,?,?,?)",(source_release,'InterHub metadata CSV',imported_at,ok,1,json.dumps(license_manifest,ensure_ascii=False)))
            dataset_id=source_dataset or license_manifest['dataset']
            connection.execute("INSERT OR REPLACE INTO source_licenses(source_dataset,source_release,code_license,dataset_license,attribution,redistribution_allowed,map_redistribution_allowed,derived_data_allowed,commercial_use_allowed,evidence_url,reviewed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)",(dataset_id,source_release,license_manifest.get('code_license'),license_manifest['dataset_license'],license_manifest['attribution'],int(license_manifest['redistribution_allowed']),int(license_manifest['map_redistribution_allowed']),int(license_manifest['derived_data_allowed']),int(license_manifest['commercial_use_allowed']),license_manifest['evidence_url'],license_manifest['reviewed_at']))
        connection.execute("PRAGMA wal_checkpoint(TRUNCATE)")
    finally:
        connection.close()
    if not resume:
        os.replace(temp_path,output_path)
    report={"release":source_release,"imported":ok,"quarantined":len(quarantined),"metadataOnly":True,"trajectoriesImported":False,"mapsImported":False}
    if quarantined:
        quarantine_path=output_path.with_suffix(output_path.suffix+'.quarantine.jsonl')
        quarantine_path.write_text(''.join(json.dumps(item,ensure_ascii=False)+'\n' for item in quarantined),encoding='utf-8')
        report['quarantineFile']=str(quarantine_path)
    return report


def build_demo_index(output_path: Path, fixture_path: Path) -> None:
    """Create a fixture-only database for local dev without mixing research rows."""
    fixture=json.loads(fixture_path.read_text(encoding='utf-8'))
    output_path.parent.mkdir(parents=True,exist_ok=True)
    connection=sqlite3.connect(output_path)
    connection.executescript(schema_sql())
    record_id=fixture['id'];release=fixture['sourceRelease']
    payload=json.dumps(fixture,ensure_ascii=False)
    with connection:
        connection.execute("INSERT OR REPLACE INTO scenarios(id,data_class,source_dataset,source_release,original_scenario_ref,track_ids_json,original_track_ids_json,agent_count,key_agent_ids_json,source_vehicle_types_json,trajectories_available,map_available,quality_flags_json,original_annotations_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)",(record_id,'synthetic-fixture','synthetic-fixture',release,record_id,json.dumps([a['id'] for a in fixture['agents']]),'[]',len(fixture['agents']),'[]','null',1,0,'[]',payload))
        connection.execute("INSERT OR REPLACE INTO releases VALUES(?,?,?,?,?,?)",(release,'deterministic local fixture',datetime.now(timezone.utc).isoformat(),1,0,json.dumps({"review_status":"fixture","dataset_license":"project synthetic fixture","attribution":"Traffic Conflict Atlas deterministic fixture","redistribution_allowed":True})))
        for agent in fixture['agents']:
            connection.execute("INSERT OR REPLACE INTO agents(scenario_id,agent_id,source_agent_id,length_m,width_m,dimension_provenance) VALUES(?,?,?,?,?,?)",(record_id,agent['id'],agent['id'],agent.get('length'),agent.get('width'),'synthetic fixture definition'))
            connection.execute("INSERT OR REPLACE INTO trajectory_assets(scenario_id,agent_id,format,uri,sha256,sample_count,time_start,time_end,coordinate_frame) VALUES(?,?,?,?,?,?,?,?,?)",(record_id,agent['id'],'json','embedded://fixture',None,len(agent.get('samples',[])),agent['samples'][0]['t'] if agent.get('samples') else None,agent['samples'][-1]['t'] if agent.get('samples') else None,'local-metric'))
    connection.close()


def main():
    parser=argparse.ArgumentParser(prog="traffic-conflicts index")
    parser.add_argument('--input',type=Path);parser.add_argument('--output',type=Path,default=Path('data/index.sqlite'));parser.add_argument('--source-release');parser.add_argument('--source');parser.add_argument('--license-manifest',type=Path);parser.add_argument('--limit',type=int);parser.add_argument('--resume',action='store_true');parser.add_argument('--demo-fixture',type=Path);parser.add_argument('--force',action='store_true')
    args=parser.parse_args()
    if args.demo_fixture:
        if args.output.exists() and not args.force: return
        build_demo_index(args.output,args.demo_fixture);print(json.dumps({"created":str(args.output),"dataClass":"synthetic-fixture"},indent=2));return
    if not args.input or not args.source_release or not args.license_manifest or not args.source: parser.error('--input, --source, --source-release, and --license-manifest are required for a research metadata index')
    license_manifest=json.loads(args.license_manifest.read_text(encoding="utf-8"))
    print(json.dumps(build_index(args.input,args.output,args.source_release,args.limit,args.resume,args.source,license_manifest),indent=2))
if __name__=='__main__': main()
