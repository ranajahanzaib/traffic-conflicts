# Data sources and licensing

## Current state

The repository contains no real-world trajectory records, maps, or InterHub release. The included `data/fixtures/synthetic-crossing.json` is a deterministic synthetic geometry fixture used to exercise replay and geometry code. It is not a research observation, is excluded from research aggregates, and must not be cited as empirical evidence.

## InterHub

The upstream InterHub repository provides extraction code under MIT. That code license does not grant rights to the source trajectory datasets, their maps, or all derived assets. Verify each source dataset's current license, attribution, redistribution and derived-data restrictions before ingesting or publishing. The current build does not mirror InterHub assets.

The upstream metadata dictionary identifies fields including dataset, scenario index, track IDs, time range, intensity, PET, key agents, path category/relation, turning label, priority label, and original source references. Preserve original labels and source identifiers when implementing an adapter.

## Sources

- [InterHub code and processing documentation](https://github.com/zxc-tju/InterHub)
- [InterHub dataset dictionary](https://github.com/zxc-tju/InterHub/blob/main/dataset.md)
- [InterHub publication](https://doi.org/10.1038/s41597-025-05344-7)
- [InterHub dataset record](https://figshare.com/articles/dataset/_/27899754)
