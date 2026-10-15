package appstate

// If is a one-line ternary operator that Go doesn't have: cond ? then_ : else_
//
//	isActive := If(deleted, nil, proto.Bool(true))
func If[T any](cond bool, then_, else_ T) T {
	if cond {
		return then_
	}
	return else_
}
