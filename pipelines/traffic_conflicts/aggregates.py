"""Exact release-level source-annotation summaries from indexed rows."""
from __future__ import annotations
import json,sqlite3
from datetime import datetime,timezone
from pathlib import Path

PET_BINS=((0,.5,"below_0_5s"),(.5,1,"0_5_to_1s"),(1,1.5,"1_to_1_5s"),(1.5,3,"1_5_to_3s"),(3,float('inf'),"3s_and_above"))

def build_aggregates(database: Path,output: Path) -> dict:
    connection=sqlite3.connect(database);connection.row_factory=sqlite3.Row
    sources=[]
    for source in connection.execute("SELECT source_dataset,count(*) n,count(source_pet) pet_n,min(source_pet) pet_min,max(source_pet) pet_max,avg(source_pet) pet_mean,count(source_intensity) intensity_n,min(source_intensity) intensity_min,max(source_intensity) intensity_max,avg(source_intensity) intensity_mean,min(agent_count) agents_min,max(agent_count) agents_max,sum(trajectories_available) trajectory_n,sum(map_available) map_n FROM scenarios WHERE data_class='research' GROUP BY source_dataset ORDER BY source_dataset"):
        pet_bins=[]
        for low,high,label in PET_BINS:
            if high==float('inf'): count=connection.execute("SELECT count(*) FROM scenarios WHERE data_class='research' AND source_dataset=? AND source_pet>=?",(source['source_dataset'],low)).fetchone()[0]
            else: count=connection.execute("SELECT count(*) FROM scenarios WHERE data_class='research' AND source_dataset=? AND source_pet>=? AND source_pet<?",(source['source_dataset'],low,high)).fetchone()[0]
            pet_bins.append({"label":label,"lowerInclusive":low,"upperExclusive":None if high==float('inf') else high,"count":count})
        sources.append({"dataset":source['source_dataset'],"scenarioCount":source['n'],"sourcePET":{"count":source['pet_n'],"min":source['pet_min'],"max":source['pet_max'],"mean":source['pet_mean'],"exploratoryBins":pet_bins},"sourceIntensity":{"count":source['intensity_n'],"min":source['intensity_min'],"max":source['intensity_max'],"mean":source['intensity_mean']},"agentCount":{"min":source['agents_min'],"max":source['agents_max']},"trajectoryScenarioCount":source['trajectory_n'],"mapScenarioCount":source['map_n']})
    total=connection.execute("SELECT count(*) FROM scenarios WHERE data_class='research'").fetchone()[0]
    release_ids=[r[0] for r in connection.execute("SELECT DISTINCT source_release FROM scenarios WHERE data_class='research' ORDER BY source_release")]
    connection.close();result={"generatedAt":datetime.now(timezone.utc).isoformat(),"researchScenarioCount":total,"releases":release_ids,"datasets":sources,"fixtureRowsExcluded":True}
    output.parent.mkdir(parents=True,exist_ok=True);output.write_text(json.dumps(result,indent=2,ensure_ascii=False)+'\n',encoding='utf-8');return result
