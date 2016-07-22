# bench

bench runs Go benchmarks in isolation.

Sometimes benchmarks influence each other and the results of a particular
benchmark are way different compared to when that benchmark is run alone. This
tool simply runs repeatedly the go test command requesting to run all selected
benchmarks one by one. The output is in the same format as that of go test.

bench does not run any tests.
