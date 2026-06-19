package goxpress

// This file layers a generics-based convenience API over the pointer-based
// binding methods on Context. Go does not allow type parameters on methods, so
// these are package-level functions taking the Context explicitly. They add no
// new binding logic: each allocates a single zero value and delegates to the
// corresponding Context method, preserving its content-type negotiation,
// struct-tag handling and 400 *HTTPError behavior.

// Bind decodes the request into a freshly allocated T, choosing JSON, form or
// query binding from the request Content-Type exactly as Context.Bind does.
//
//	req, err := goxpress.Bind[CreateUser](c)
func Bind[T any](c *Context) (T, error) {
	var v T
	err := c.Bind(&v)
	return v, err
}

// BindJSON decodes the JSON request body into a freshly allocated T.
//
//	req, err := goxpress.BindJSON[CreateUser](c)
func BindJSON[T any](c *Context) (T, error) {
	var v T
	err := c.BindJSON(&v)
	return v, err
}

// BindQuery binds the URL query into a freshly allocated T using "query" struct
// tags.
//
//	filter, err := goxpress.BindQuery[Filter](c)
func BindQuery[T any](c *Context) (T, error) {
	var v T
	err := c.BindQuery(&v)
	return v, err
}

// BindForm parses and binds form fields into a freshly allocated T using "form"
// struct tags.
//
//	login, err := goxpress.BindForm[LoginRequest](c)
func BindForm[T any](c *Context) (T, error) {
	var v T
	err := c.BindForm(&v)
	return v, err
}
