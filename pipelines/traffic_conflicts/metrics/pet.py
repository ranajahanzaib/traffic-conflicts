"""Small, auditable geometry helpers; coordinates use caller-supplied metric units."""
from __future__ import annotations
from dataclasses import dataclass
from typing import Iterable

@dataclass(frozen=True)
class Sample:
    t: float
    x: float
    y: float

@dataclass(frozen=True)
class Occupancy:
    entry: float
    exit: float


def occupancies(samples: Iterable[Sample], center: tuple[float, float], radius: float) -> list[Occupancy]:
    """Find sampled center-point occupancy intervals in a circular zone.

    Segment/zone intersections are linearly interpolated, including pass-throughs
    with no sample inside. Intervals are merged only when adjacent segments meet
    in-zone. Callers must split trajectories at missing-data gaps before calling.
    """
    points = list(samples)
    if not _finite(radius) or radius <= 0 or len(points) < 2:
        raise ValueError("radius must be positive and at least two samples are required")
    if any(not all(map(_finite, (p.t, p.x, p.y))) for p in points):
        raise ValueError("sample coordinates and timestamps must be finite")
    if any(b.t <= a.t for a, b in zip(points, points[1:])):
        raise ValueError("timestamps must be strictly increasing")
    cx, cy = center
    if not all(map(_finite, (cx, cy))):
        raise ValueError("zone center must be finite")
    result: list[Occupancy] = []
    for a, b in zip(points, points[1:]):
        dx, dy = b.x-a.x, b.y-a.y
        fx, fy = a.x-cx, a.y-cy
        aa = dx*dx + dy*dy
        if aa == 0:
            if fx*fx + fy*fy <= radius*radius:
                _append_interval(result, Occupancy(a.t,b.t))
            continue
        bb = 2*(fx*dx + fy*dy)
        cc = fx*fx + fy*fy - radius*radius
        disc = bb*bb - 4*aa*cc
        roots = [] if disc < 0 else [(-bb-disc**0.5)/(2*aa),(-bb+disc**0.5)/(2*aa)]
        cuts = [0.0] + sorted({max(0.0,min(1.0,r)) for r in roots if 0 <= r <= 1}) + [1.0]
        for lo, hi in zip(cuts,cuts[1:]):
            mid=(lo+hi)/2
            mx,my=fx+mid*dx,fy+mid*dy
            if mx*mx+my*my <= radius*radius + 1e-12:
                _append_interval(result,Occupancy(a.t+lo*(b.t-a.t),a.t+hi*(b.t-a.t)))
        # A tangent can touch the zone without an inside segment of positive length.
        if disc == 0 and roots and 0 <= roots[0] <= 1:
            u=roots[0]; t=a.t+u*(b.t-a.t); _append_interval(result,Occupancy(t,t))
    return result


def occupancy(samples: Iterable[Sample], center: tuple[float, float], radius: float) -> Occupancy | None:
    """Return the first occupancy interval, or None if the path misses the zone."""
    intervals=occupancies(samples,center,radius)
    return intervals[0] if intervals else None


def sequential_pet(first: Occupancy, second: Occupancy) -> float | None:
    """PET for non-overlapping sequential occupancy; simultaneous is undefined."""
    if second.entry < first.exit:
        return None
    return second.entry - first.exit


def _append_interval(intervals: list[Occupancy], interval: Occupancy) -> None:
    if intervals and interval.entry <= intervals[-1].exit + 1e-12:
        intervals[-1]=Occupancy(intervals[-1].entry,max(intervals[-1].exit,interval.exit))
    else:
        intervals.append(interval)


def _finite(value: float) -> bool:
    return value == value and abs(value) != float("inf")
