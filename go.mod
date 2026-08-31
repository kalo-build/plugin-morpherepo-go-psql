module github.com/kalo-build/plugin-morpherepo-go-psql

go 1.21.6

require (
	github.com/kalo-build/go-util v0.0.0-20250329083327-00e97aeff9b7
	github.com/kalo-build/morphe-go v0.0.0-20260312144523-0841ef1ee9f8
	github.com/kalo-build/plugin-morpherepo-go v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.9.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kalo-build/clone v0.0.0-20250329082958-41db0353412f // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/kalo-build/plugin-morpherepo-go => ../plugin-morpherepo-go
