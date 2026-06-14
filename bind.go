package goxpress

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// Bind deserializes the request into v, choosing the strategy from the request
// Content-Type: JSON bodies are decoded as JSON, form bodies are bound by
// "form" struct tags. When no body content type matches, the URL query is
// bound by "query" struct tags.
//
// Binding failures are returned as a 400 *HTTPError, so a handler can simply
// `return c.Bind(&v)` and let the error handler produce the response.
func (c *Context) Bind(v any) error {
	ct := c.contentType()
	switch {
	case strings.HasPrefix(ct, "application/json"):
		return c.BindJSON(v)
	case strings.HasPrefix(ct, "application/x-www-form-urlencoded"),
		strings.HasPrefix(ct, "multipart/form-data"):
		return c.BindForm(v)
	default:
		return c.BindQuery(v)
	}
}

// BindJSON decodes the request body into v as JSON.
func (c *Context) BindJSON(v any) error {
	if c.Request.Body == nil {
		return NewHTTPError(http.StatusBadRequest, "empty request body")
	}
	dec := json.NewDecoder(c.Request.Body)
	if err := dec.Decode(v); err != nil {
		return NewHTTPError(http.StatusBadRequest, "invalid JSON body").WithInternal(err)
	}
	return nil
}

// BindQuery binds URL query parameters into v using "query" struct tags.
func (c *Context) BindQuery(v any) error {
	if err := bindValues(v, c.queryValues(), "query"); err != nil {
		return NewHTTPError(http.StatusBadRequest, "invalid query parameters").WithInternal(err)
	}
	return nil
}

// BindForm parses and binds form fields into v using "form" struct tags.
func (c *Context) BindForm(v any) error {
	if err := c.Request.ParseForm(); err != nil {
		return NewHTTPError(http.StatusBadRequest, "invalid form body").WithInternal(err)
	}
	if err := bindValues(v, c.Request.Form, "form"); err != nil {
		return NewHTTPError(http.StatusBadRequest, "invalid form fields").WithInternal(err)
	}
	return nil
}

// contentType returns the request's Content-Type without any parameters.
func (c *Context) contentType() string {
	ct := c.Request.Header.Get("Content-Type")
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(ct)
}

// bindValues maps the named values into the exported fields of the struct
// pointed to by ptr, matching each field by its struct tag (or field name when
// the tag is absent).
func bindValues(ptr any, values map[string][]string, tag string) error {
	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &bindError{"bind target must be a non-nil pointer to a struct"}
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return &bindError{"bind target must point to a struct"}
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		name := field.Tag.Get(tag)
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}

		vals, ok := values[name]
		if !ok || len(vals) == 0 {
			continue
		}
		if err := setField(rv.Field(i), vals); err != nil {
			return err
		}
	}
	return nil
}

// setField assigns the string value(s) to a struct field, converting to the
// field's kind. Slice fields receive every value; scalar fields receive the
// first.
func setField(field reflect.Value, values []string) error {
	if field.Kind() == reflect.Slice {
		slice := reflect.MakeSlice(field.Type(), len(values), len(values))
		for i, v := range values {
			if err := setScalar(slice.Index(i), v); err != nil {
				return err
			}
		}
		field.Set(slice)
		return nil
	}
	return setScalar(field, values[0])
}

// setScalar parses s into the scalar destination according to its kind.
func setScalar(dst reflect.Value, s string) error {
	switch dst.Kind() {
	case reflect.String:
		dst.SetString(s)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return &bindError{"invalid bool value: " + s}
		}
		dst.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, dst.Type().Bits())
		if err != nil {
			return &bindError{"invalid integer value: " + s}
		}
		dst.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, dst.Type().Bits())
		if err != nil {
			return &bindError{"invalid unsigned integer value: " + s}
		}
		dst.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, dst.Type().Bits())
		if err != nil {
			return &bindError{"invalid float value: " + s}
		}
		dst.SetFloat(f)
	default:
		return &bindError{"unsupported field type: " + dst.Kind().String()}
	}
	return nil
}

// bindError is a simple internal binding error carrying a human-readable
// message.
type bindError struct{ msg string }

func (e *bindError) Error() string { return "goxpress: " + e.msg }
