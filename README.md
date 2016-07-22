# bench

bench runs Go benchmarks in isolation.

Sometimes benchmarks influence each other and the results of a particular
benchmark are way different compared to when that benchmark is run alone.  A
common, but definitely not the only cause of this is the interference of the
garbage collector where the previous benchmark stressed the memory usage
substantially. Manually invoking the GC doesn't seem to always help as it looks
like it's only a hint to the runtime.  This tool simply runs repeatedly the go
test command requesting to run all package benchmarks one by one. The output is
in a format similar to go test and it should be benchcmp compatible.

bench does not run any tests.

**Example**

```
$ benchcmp -mag -changed log-go-test log-bench
benchmark                                         old ns/op     new ns/op     delta
BenchmarkAllocatorRndGetSimpleFileFiler1e3-4      418091        2217          -99.47%
BenchmarkAllocatorRndFreeSimpleFileFiler1e3-4     31709         8038          -74.65%
BenchmarkAllocatorAllocSimpleFileFiler1e3-4       14292         5675          -60.29%
...
```
