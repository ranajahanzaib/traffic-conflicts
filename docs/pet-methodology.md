# PET measurement notes

Post-encroachment time is defined here for sequential use of a shared conflict region as the second entry timestamp minus the first exit timestamp. A negative or overlapping occupancy is not silently converted to zero; simultaneous/overlapping occupancy returns an undefined sequential PET and should be reported as a separate condition.

The Python starter metric module currently supports a center-point circular region and linear interpolation between adjacent valid samples. It validates strictly increasing timestamps. It does not bridge missing intervals, infer unknown dimensions, reproduce InterHub's published source PET, or calculate vehicle-footprint PET. Its fixtures are synthetic.

Every derived measurement should record the zone geometry, agent order, entry and exit times, coordinate frame, sample resolution, interpolation method, and dimension provenance. PET alone is not a probability of collision or a universal risk category.
