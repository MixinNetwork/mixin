package kernel

func panicGo(f func() error) {
	go func() {
		if err := f(); err != nil {
			panic(err)
		}
	}()
}
