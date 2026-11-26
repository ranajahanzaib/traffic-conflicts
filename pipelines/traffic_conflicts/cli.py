"""Reproducible local CLI for inspecting and indexing user-provided InterHub metadata."""
import argparse, csv, json, sqlite3
from pathlib import Path
from pipelines.traffic_conflicts.adapters.interhub import REQUIRED_COLUMNS
from pipelines.traffic_conflicts.indexing import build_index
from pipelines.traffic_conflicts.aggregates import build_aggregates
from pipelines.traffic_conflicts.adapters.interhub import iter_metadata
from pipelines.traffic_conflicts.normalization import normalize_interhub_row

DATASETS=("nuplan_train","waymo_train","interaction_single","interaction_multi","lyft_train_full","av2_motion_forecasting")
COMMANDS=("sources-list","sources-inspect","download","ingest","normalize","validate","compute-metrics","build-index","build-aggregates","publish","verify-release")

def inspect_csv(path: Path) -> dict:
    with path.open(newline="",encoding="utf-8-sig") as stream:
        reader=csv.DictReader(stream)
        columns=reader.fieldnames or []
        missing=REQUIRED_COLUMNS-set(columns)
        sample=next(reader,None)
    return {"columns":columns,"missingRequiredColumns":sorted(missing),"validSchema":not missing,"sampleAvailable":sample is not None}

def main():
    parser=argparse.ArgumentParser(prog="traffic-conflicts")
    parser.add_argument("command",choices=COMMANDS)
    parser.add_argument("--source",choices=DATASETS)
    parser.add_argument("--source-release")
    parser.add_argument("--license-manifest",type=Path)
    parser.add_argument("--input",type=Path)
    parser.add_argument("--output",type=Path)
    parser.add_argument("--scenario-limit","--limit",dest="scenario_limit",type=int)
    
    parser.add_argument("--resume",action="store_true")
    args=parser.parse_args()
    if args.command=="sources-list":
        print(json.dumps([{"id":d,"status":"adapter-supported; not necessarily imported"} for d in DATASETS],indent=2));return
    if args.command=="sources-inspect":
        if not args.input or not args.input.is_file(): parser.error("--input must point to a local metadata CSV")
        report=inspect_csv(args.input);print(json.dumps(report,indent=2));raise SystemExit(0 if report['validSchema'] else 2)
    if args.command=="normalize":
        if not args.input or not args.input.is_file(): parser.error("--input must point to a local metadata CSV")
        if not args.source_release: parser.error("--source-release is required for provenance")
        if not args.output: parser.error("--output must identify a JSONL destination")
        count=0;args.output.parent.mkdir(parents=True,exist_ok=True)
        with args.output.open('w',encoding='utf-8') as destination:
            for record in iter_metadata(args.input,args.source_release,args.scenario_limit): destination.write(json.dumps(record.as_dict(),ensure_ascii=False)+'\n');count+=1
        print(json.dumps({"normalized":count,"output":str(args.output)},indent=2));return
    if args.command=="validate":
        if not args.input or not args.input.is_file(): parser.error("--input must point to a local metadata CSV")
        if not args.source_release: parser.error("--source-release is required for provenance")
        schema=inspect_csv(args.input)
        if not schema['validSchema']: print(json.dumps(schema,indent=2));raise SystemExit(2)
        with args.input.open(newline="",encoding="utf-8-sig") as stream:
            records=[normalize_interhub_row(row,args.source_release) for i,row in enumerate(csv.DictReader(stream)) if args.scenario_limit is None or i<args.scenario_limit]
        warnings=sum(bool(record.quality_flags) for record in records)
        print(json.dumps({"rows":len(records),"recordsWithQualityFlags":warnings,"schemaValid":True,"sourceRelease":args.source_release},indent=2));return
    if args.command in {"ingest","build-index"}:
        if not args.input or not args.input.is_file(): parser.error("--input must point to the local InterHub metadata CSV")
        if not args.source_release: parser.error("--source-release is required to preserve provenance")
        if not args.source: parser.error("--source must select one dataset for source-specific license review")
        if not args.license_manifest or not args.license_manifest.is_file(): parser.error("--license-manifest must point to the reviewed source-terms JSON")
        license_manifest=json.loads(args.license_manifest.read_text(encoding="utf-8"))
        output=args.output or Path("data/index.sqlite")
        print(json.dumps(build_index(args.input,output,args.source_release,args.scenario_limit,args.resume,args.source,license_manifest),indent=2));return
    if args.command=="build-aggregates":
        source=args.input or Path('data/index.sqlite')
        output=args.output or source.with_suffix('.aggregates.json')
        try: result=build_aggregates(source,output)
        except (sqlite3.Error,FileNotFoundError) as exc: parser.error(f"cannot aggregate release index: {exc}")
        print(json.dumps({"output":str(output),"researchScenarioCount":result['researchScenarioCount']},indent=2));return
    if args.command in {"download","compute-metrics","publish","verify-release"}:
        reasons={"download":"Network access is unavailable; place a rights-checked source release locally.","compute-metrics":"No trajectory cache is available; metadata alone cannot produce derived motion metrics.","publish":"No publishing configuration is set; licensing and destination checks are required.","verify-release":"No release bundle exists yet."}
        parser.error(reasons[args.command])
if __name__=="__main__": main()
