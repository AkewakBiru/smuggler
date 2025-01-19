package smuggler

type H2 struct {
	DesyncerImpl
}

func (h2 *H2) Run() bool {
	return false
}
