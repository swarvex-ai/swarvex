module github.com/swarvex-ai/swarvex/pipeline

go 1.27

require github.com/swarvex-ai/swarvex/runner v0.0.0

// Das gemeinsame Paket umgebung liegt im Runner-Modul desselben Repos.
replace github.com/swarvex-ai/swarvex/runner => ../runner
