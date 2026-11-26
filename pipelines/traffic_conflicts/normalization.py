"""Normalized InterHub metadata records. Original source values remain attached."""
from __future__ import annotations
from dataclasses import asdict, dataclass
import hashlib, json, math
from typing import Any

@dataclass(frozen=True)
class ScenarioRecord:
    scenario_id: str
    source_dataset: str
    source_release: str
    original_scenario_ref: str | None
    original_scene_id: str | None
    original_data_file: str | None
    source_cache_folder: str | None
    source_scenario_index: str | None
    source_track_ids: tuple[str,...]
    original_track_ids: tuple[str,...]
    interaction_start: float | None
    interaction_end: float | None
    source_pet: float | None
    source_intensity: float | None
    agent_count: int
    agent_count_label: str | None
    key_agent_ids: tuple[str,...]
    av_involvement: str | None
    source_vehicle_types: Any
    source_path_category: str | None
    source_path_relation: str | None
    key_pair_turns: tuple[str,...]
    priority_agent_id: str | None
    trajectories_available: bool
    map_available: bool
    quality_flags: tuple[str,...]
    original_annotations: dict[str,Any]

    def as_dict(self) -> dict[str,Any]:
        return asdict(self)


def normalize_interhub_row(row: dict[str,str], source_release: str) -> ScenarioRecord:
    """Normalize one row while preserving unknown labels and raw source values."""
    dataset=_text(row.get('dataset'))
    scene=_text(row.get('original_scene_id'))
    idx=_text(row.get('scenario_idx'))
    source_file=_text(row.get('original_data_file'))
    cache_folder=_text(row.get('folder'))
    ref=scene or source_file or (f'{dataset}:{idx}' if dataset and idx else None)
    identity={'dataset':dataset,'scene':scene,'file':source_file,'index':idx,
              'start':_text(row.get('start')),'end':_text(row.get('end')),
              'tracks':_split(row.get('track_id'))}
    scenario_id='ih-'+hashlib.sha256(json.dumps(identity,sort_keys=True,separators=(',',':')).encode()).hexdigest()[:24]
    flags=[]
    if not dataset: flags.append('missing_source_dataset')
    if not ref: flags.append('missing_original_scenario_reference')
    start=_number(row.get('start'),'interaction_start',flags)
    end=_number(row.get('end'),'interaction_end',flags)
    if start is not None and end is not None and end < start: flags.append('interaction_end_before_start')
    tracks=_split(row.get('track_id')); key_agents=_split(row.get('key_agents'))
    if len(set(key_agents)) != len(key_agents): flags.append('duplicate_key_agent_reference')
    if len(key_agents) != 2: flags.append('key_pair_does_not_contain_two_agents')
    if any(agent not in tracks for agent in key_agents): flags.append('key_agent_not_in_track_list')
    turns=tuple(_split(row.get('turn_label'),'-'))
    if turns and len(turns)!=2: flags.append('turn_label_pair_malformed')
    priority=_text(row.get('priority_label'))
    if priority and key_agents and priority not in key_agents: flags.append('priority_agent_not_in_key_pair')
    agent_label=_text(row.get('two/multi'))
    declared_count=2 if agent_label=='two' else None
    if agent_label=='multi':
        declared_count=len(tracks)
    if not agent_label: flags.append('missing_source_agent_count_label')
    # Metadata rows identify interacting agents but do not carry individual track dimensions.
    return ScenarioRecord(
        scenario_id=scenario_id,source_dataset=dataset or 'unknown',source_release=source_release,
        original_scenario_ref=ref,original_scene_id=scene,original_data_file=source_file,
        source_cache_folder=cache_folder,source_scenario_index=idx,
        source_track_ids=tracks,original_track_ids=_split(row.get('original_track_id')),
        interaction_start=start,interaction_end=end,
        source_pet=_number(row.get('PET'),'source_pet',flags),
        source_intensity=_number(row.get('intensity'),'source_intensity',flags),
        agent_count=declared_count if declared_count is not None else len(tracks),
        agent_count_label=agent_label,key_agent_ids=key_agents,
        av_involvement=_text(row.get('AV_included')),
        source_vehicle_types=_parse_source_value(row.get('vehicle_type')),
        source_path_category=_text(row.get('path_category')),
        source_path_relation=_text(row.get('path_relation')),
        key_pair_turns=turns,priority_agent_id=priority,
        trajectories_available=False,map_available=False,
        quality_flags=tuple(dict.fromkeys(flags)),original_annotations=dict(row))


def _split(value: str|None, delimiter: str=';') -> tuple[str,...]:
    if not value or not value.strip(): return ()
    return tuple(part.strip() for part in value.split(delimiter) if part.strip())

def _text(value: str|None) -> str|None:
    if value is None or not value.strip() or value.strip().lower() in {'nan','null','none'}: return None
    return value.strip()

def _number(value: str|None,field: str,flags: list[str]) -> float|None:
    if value is None or not value.strip() or value.strip().lower() in {'null','none'}: return None
    try: result=float(value.strip())
    except ValueError: flags.append(f'invalid_{field}'); return None
    if not math.isfinite(result): flags.append(f'nonfinite_{field}'); return None
    return result

def _parse_source_value(value: str|None) -> Any:
    """Parse Python-list-like source values without executing input code."""
    value=_text(value)
    if value is None: return None
    if value.startswith('[') and value.endswith(']'):
        inner=value[1:-1].strip()
        if not inner: return []
        return [part.strip().strip("'\"") for part in inner.split(',')]
    return value
