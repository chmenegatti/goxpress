package goxpress

// Param is a single URL parameter, consisting of a key and a value.
type Param struct {
	Key   string
	Value string
}

// Params is an ordered list of URL parameters captured while matching a route.
//
// The order mirrors the order in which the parameters appear in the route
// pattern, which makes lookups cheap for the small number of parameters a
// typical route declares.
type Params []Param

// Get returns the value of the first parameter whose key matches name and
// reports whether it was found.
func (ps Params) Get(name string) (string, bool) {
	for i := range ps {
		if ps[i].Key == name {
			return ps[i].Value, true
		}
	}
	return "", false
}

// ByName returns the value of the first parameter whose key matches name, or
// the empty string if no such parameter exists.
func (ps Params) ByName(name string) string {
	v, _ := ps.Get(name)
	return v
}
