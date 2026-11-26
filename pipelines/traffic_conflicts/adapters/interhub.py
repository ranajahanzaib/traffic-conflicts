"""InterHub CSV metadata adapter; it does not claim trajectory/map availability."""
from __future__ import annotations
import csv
from pathlib import Path
from typing import Iterator
from pipelines.traffic_conflicts.normalization import ScenarioRecord, normalize_interhub_row

REQUIRED_COLUMNS={"dataset","folder","scenario_idx","track_id","start","end","intensity","PET","two/multi","vehicle_type","AV_included","key_agents","path_category","path_relation","turn_label","priority_label","original_data_file","original_scene_id","original_track_id"}

def iter_metadata(path: Path, source_release: str, limit: int|None=None) -> Iterator[ScenarioRecord]:
    with path.open(newline="",encoding="utf-8-sig") as stream:
        reader=csv.DictReader(stream)
        missing=REQUIRED_COLUMNS-set(reader.fieldnames or [])
        if missing: raise ValueError(f"metadata CSV is missing required columns: {', '.join(sorted(missing))}")
        for idx,row in enumerate(reader):
            if limit is not None and idx>=limit: break
            yield normalize_interhub_row(row,source_release)
