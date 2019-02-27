package common

const (
	NodeStatePledging  = "PLEDGING"
	NodeStateAccepted  = "ACCEPTED"
	NodeStateDeparting = "DEPARTING"
)

type Node struct {
	Signer Address
	Payee  Address
	State  string
}

func (n *Node) IsAccepted() bool {
	return n.State == NodeStateAccepted
}
