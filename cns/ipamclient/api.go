package ipamclient

const (
	getAddressSpacesPath = "/IpamDriver.GetDefaultAddressSpaces"
	requestPoolPath      = "/IpamDriver.RequestPool"
	reserveAddrPath      = "/IpamDriver.RequestAddress"
	releaseAddrPath      = "/IpamDriver.ReleaseAddress"
	getPoolInfoPath      = "/IpamDriver.GetPoolInfo"
)

// Config describes subnet/gateway for ipam.
type Config struct {
	Subnet string
}

type getAddressSpacesResponse struct {
	LocalDefaultAddressSpace  string
	GlobalDefaultAddressSpace string
}

// Request sent by libnetwork when acquiring a reference to an address pool.
type requestPoolRequest struct {
	AddressSpace string
	Pool         string
	SubPool      string
	Options      map[string]string
	V6           bool
}

// Response sent by plugin when an address pool is successfully referenced.
type requestPoolResponse struct {
	PoolID string
	Pool   string
	Data   map[string]string
}

// NetworkConfiguration describes configuration for docker network create.
type reserveAddrRequest struct {
	PoolID  string
	Address string
	Options map[string]string
}

// DockerErrorResponse defines the error response retunred by docker.
type reserveAddrResponse struct {
	Address string
}

// NetworkConfiguration describes configuration for docker network create.
type releaseAddrRequest struct {
	PoolID  string
	Address string
	Options map[string]string
}

type releaseAddrResponse struct {
	Err string
}

// Request sent when querying address pool information.
type getPoolInfoRequest struct {
	PoolID string
}

// Response sent by plugin when returning address pool information.
type getPoolInfoResponse struct {
	Capacity           int
	Available          int
	UnhealthyAddresses []string
}
