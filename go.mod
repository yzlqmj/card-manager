module card-manager

go 1.23.4

replace card-manager/localizer => ./localizer

require (
	card-manager/localizer v0.0.0-00010101000000-000000000000
	github.com/lmittmann/tint v1.1.2
)

require gopkg.in/yaml.v3 v3.0.1 // indirect
