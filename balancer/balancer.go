package balancer

type Balancer interface {
	Get() (Connection, error)
	Connections() []Connection
}
