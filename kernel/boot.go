package kernel

func (node *Node) Loop() error {
	panicGo(node.ListenNeighbors)
	panicGo(node.ConsumeMempool)
	panicGo(node.LoadCacheToQueue)
	panicGo(node.MintLoop)
	panicGo(node.ElectionLoop)
	return node.ConsumeQueue()
}
