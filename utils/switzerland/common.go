package switzerland

type Switzerland interface {
	Notify(chan<- WinchSignal)
	Stop(chan<- WinchSignal)
}

func GetSwitzerland() Switzerland {
	return switzerlandInstance
}
