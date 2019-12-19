module github.com/sagikazarmark/ocwatermill/_example

go 1.12

require (
	contrib.go.opencensus.io/exporter/prometheus v0.1.0
	github.com/ThreeDotsLabs/watermill v1.0.3
	github.com/go-chi/chi v4.0.2+incompatible
	github.com/sagikazarmark/ocwatermill v0.0.0-00010101000000-000000000000
	go.opencensus.io v0.22.2
)

// uncomment to use local sources
replace github.com/sagikazarmark/ocwatermill => ../
