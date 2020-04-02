module github.com/Azure/azure-container-networking/npm

go 1.13

replace (
	github.com/Azure/azure-container-networking => ../
	k8s.io/api => k8s.io/api v0.16.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.8
	k8s.io/client-go => k8s.io/client-go v0.16.8
)

require (
	github.com/Azure/azure-container-networking v1.0.33
	github.com/Masterminds/semver v1.5.0
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00 // indirect
	golang.org/x/sys v0.0.0-20191120155948-bd437916bb0e
	k8s.io/api v0.18.0
	k8s.io/apimachinery v0.18.0
	k8s.io/client-go v0.18.0
)
