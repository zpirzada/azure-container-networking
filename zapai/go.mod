module github.com/Azure/azure-container-networking/zapai

go 1.19

require (
	github.com/Azure/azure-container-networking v1.4.35
	github.com/jsternberg/zap-logfmt v1.3.0
	github.com/microsoft/ApplicationInsights-Go v0.4.4
	github.com/pkg/errors v0.9.1
	go.uber.org/zap v1.23.0
)

require (
	code.cloudfoundry.org/clock v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/Microsoft/hcsshim v0.8.23 // indirect
	github.com/containerd/cgroups v1.0.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gofrs/uuid v4.2.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/stretchr/testify v1.8.0 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/goleak v1.1.12 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9 // indirect
)

replace github.com/Microsoft/hcsshim => github.com/vakalapa/hcsshim v0.9.1-0.20211203205307-837d4d06df77
