Run the performance optimization workflow:

1. Run benchmarks for Go services to establish a baseline:
   - `cd basket-service && go test -bench=. -benchmem ./...`
   - `cd onboarding && go test -bench=. -benchmem ./...`
2. Show the current benchmark results
3. Ask what to optimize (or analyze the code to suggest optimizations)
4. After I approve changes, implement the optimization
5. Re-run the same benchmarks to capture new results
6. Compare before/after results
7. Run `make test` to verify correctness is preserved
8. Summarize the performance improvement with before/after numbers
