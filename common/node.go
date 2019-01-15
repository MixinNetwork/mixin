package common

const (
	NodeStatePledging  = "PLEDGING"
	NodeStateAccepted  = "ACCEPTED"
	NodeStateDeparting = "DEPARTING"
)

type Node struct {
	Account Address
	State   string
}

func (n *Node) IsAccepted() bool {
	return n.State == NodeStateAccepted
}
