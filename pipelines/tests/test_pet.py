import unittest
from pipelines.traffic_conflicts.metrics.pet import Sample, Occupancy, occupancy, occupancies, sequential_pet

class PetTests(unittest.TestCase):
    def test_interpolates_circle_boundary(self):
        result=occupancy([Sample(0,-2,0),Sample(1,0,0),Sample(2,2,0)],(0,0),1)
        self.assertAlmostEqual(result.entry,.5)
        self.assertAlmostEqual(result.exit,1.5)
    def test_sequential_pet(self):
        self.assertAlmostEqual(sequential_pet(Occupancy(1,2),Occupancy(3.5,4)),1.5)
    def test_simultaneous_or_overlapping_occupancy_is_undefined(self):
        self.assertIsNone(sequential_pet(Occupancy(1,2),Occupancy(1.8,3)))
    def test_pass_through_between_samples_is_detected(self):
        result=occupancy([Sample(0,-2,0),Sample(1,2,0)],(0,0),1)
        self.assertAlmostEqual(result.entry,.25)
        self.assertAlmostEqual(result.exit,.75)
    def test_repeated_zone_visits_remain_separate(self):
        result=occupancies([Sample(0,-2,0),Sample(1,2,0),Sample(2,-2,0),Sample(3,2,0)],(0,0),1)
        self.assertEqual(len(result),3)
        self.assertAlmostEqual(result[1].entry,1.25)
        self.assertAlmostEqual(result[1].exit,1.75)
    def test_non_intersecting_paths(self):
        self.assertIsNone(occupancy([Sample(0,-2,8),Sample(1,2,8)],(0,0),1))
    def test_rejects_nonmonotonic_timestamps(self):
        with self.assertRaises(ValueError): occupancy([Sample(1,0,0),Sample(1,1,1)],(0,0),1)
    def test_rejects_invalid_zone(self):
        with self.assertRaises(ValueError): occupancy([Sample(0,0,0),Sample(1,1,1)],(0,0),0)

if __name__=='__main__': unittest.main()
