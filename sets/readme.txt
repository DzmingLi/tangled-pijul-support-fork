sets
----
set datastructure for go with generics and iterators. the
api is supposed to mimic rust's std::collections::HashSet api.

    s1 := sets.Collect(slices.Values([]int{1, 2, 3, 4}))
    s2 := sets.Collect(slices.Values([]int{1, 2, 3, 4, 5, 6}))

    union     := sets.Collect(s1.Union(s2))
    intersect := sets.Collect(s1.Intersection(s2))
    diff      := sets.Collect(s1.Difference(s2))
    symdiff   := sets.Collect(s1.SymmetricDifference(s2))

    s1.Len()          // 4
    s1.Contains(1)    // true
    s1.IsEmpty()      // false
    s1.IsSubset(s2)   // true
    s1.IsSuperset(s2) // false
    s1.IsDisjoint(s2) // false

    if exists := s1.Insert(1); exists {
        // already existed in set
    }

    if existed := s1.Remove(1); existed {
        // existed in set, now removed
    }


testing
-------
includes property-based tests using the wonderful
testing/quick module!

    go test -v
