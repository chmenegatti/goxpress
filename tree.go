package goxpress

// nodeType classifies a radix-tree node by the kind of path segment it matches.
type nodeType uint8

const (
	// nodeStatic matches a literal substring of the path.
	nodeStatic nodeType = iota
	// nodeRoot is the root of a per-method tree.
	nodeRoot
	// nodeParam matches a single path segment and captures it (":name").
	nodeParam
	// nodeCatchAll matches the remainder of the path and captures it ("*name").
	nodeCatchAll
)

// node is a single node of the priority-ordered radix tree.
//
// Static children are looked up through the indices string: indices[i] is the
// first byte of children[i].path, allowing a fast byte comparison before
// descending. A node has at most one wildcard child (param or catch-all),
// flagged by wildChild and stored as the last child.
type node struct {
	path      string  // path segment owned by this node
	indices   string  // first byte of every static child, in child order
	children  []*node // child nodes, static ones first
	handlers  []HandlerFunc
	paramName string   // captured name for param / catch-all nodes
	priority  uint32   // number of routes registered through this node
	nType     nodeType // node classification
	wildChild bool     // whether the last child is a wildcard node

	// matcher constrains a param node: when non-nil, a captured segment that
	// fails it does not match the route. It is nil for unconstrained params, so
	// the unconstrained hot path pays nothing.
	matcher ParamMatcher
}

// addRoute registers handlers for the given path within the tree rooted at n.
// It panics on malformed patterns or conflicts, which are programming errors
// that must surface at startup rather than at request time.
func (n *node) addRoute(path string, handlers []HandlerFunc, resolve matcherResolver) {
	fullPath := path
	n.priority++

	// Empty tree: insert the whole path as the first route.
	if n.path == "" && len(n.children) == 0 {
		n.insertChild(path, fullPath, handlers, resolve)
		n.nType = nodeRoot
		return
	}

walk:
	for {
		// Find the longest common prefix between path and this node's path.
		i := longestCommonPrefix(path, n.path)

		// Split the current node when the common prefix is shorter than its
		// path: the existing suffix becomes a child of a new shared parent.
		if i < len(n.path) {
			child := &node{
				path:      n.path[i:],
				indices:   n.indices,
				children:  n.children,
				handlers:  n.handlers,
				paramName: n.paramName,
				priority:  n.priority - 1,
				nType:     nodeStatic,
				wildChild: n.wildChild,
			}

			n.children = []*node{child}
			n.indices = string(n.path[i])
			n.path = path[:i]
			n.handlers = nil
			n.paramName = ""
			n.wildChild = false
		}

		// Descend into (or create) a child for the remaining path.
		if i < len(path) {
			path = path[i:]
			c := path[0]

			// A param child can continue with the part after the segment.
			if n.nType == nodeParam && c == '/' && len(n.children) == 1 {
				n = n.children[0]
				n.priority++
				continue walk
			}

			// Match an existing static child by its first byte.
			if n.nType != nodeParam {
				for idx := 0; idx < len(n.indices); idx++ {
					if c == n.indices[idx] {
						idx = n.incrementChildPriority(idx)
						n = n.children[idx]
						continue walk
					}
				}
			}

			// No matching child: insert one, unless a wildcard begins here.
			if c != ':' && c != '*' && n.nType != nodeCatchAll {
				n.indices += string(c)
				child := &node{}
				n.children = append(n.children, child)
				n.incrementChildPriority(len(n.indices) - 1)
				n = child
			} else if n.wildChild {
				// Descend into the existing wildcard child and validate it is
				// compatible with the new route.
				n = n.children[len(n.children)-1]
				n.priority++

				if len(path) >= len(n.path) &&
					n.path == path[:len(n.path)] &&
					n.nType != nodeCatchAll &&
					(len(n.path) >= len(path) || path[len(n.path)] == '/') {
					continue walk
				}

				panic("wildcard conflict: '" + path +
					"' conflicts with existing route in path '" + fullPath + "'")
			}

			n.insertChild(path, fullPath, handlers, resolve)
			return
		}

		// The path is fully consumed by this node: attach the handlers here.
		if n.handlers != nil {
			panic("handlers are already registered for path '" + fullPath + "'")
		}
		n.handlers = handlers
		return
	}
}

// insertChild grows the tree from n along path, creating static, param and
// catch-all nodes as dictated by the wildcards it contains.
func (n *node) insertChild(path, fullPath string, handlers []HandlerFunc, resolve matcherResolver) {
	for {
		// Locate the next wildcard segment, if any.
		wildcard, idx, valid := findWildcard(path)
		if idx < 0 { // no wildcard left
			break
		}

		if !valid {
			panic("only one wildcard per path segment is allowed, has: '" +
				wildcard + "' in path '" + fullPath + "'")
		}
		if len(wildcard) < 2 {
			panic("wildcards must be named with a non-empty name in path '" +
				fullPath + "'")
		}

		if wildcard[0] == ':' { // param segment
			// Emit the static prefix before the wildcard, if any.
			if idx > 0 {
				n.path = path[:idx]
				path = path[idx:]
			}

			name, matcher := parseParam(wildcard, resolve, fullPath)
			child := &node{path: wildcard, nType: nodeParam, paramName: name, matcher: matcher}
			n.children = []*node{child}
			n.wildChild = true
			n = child
			n.priority++

			// Continue with the remainder after this segment, if present.
			if len(wildcard) < len(path) {
				path = path[len(wildcard):]
				child := &node{priority: 1}
				n.children = []*node{child}
				n.indices = string(path[0])
				n = child
				continue
			}

			n.handlers = handlers
			return
		}

		// Catch-all segment ("*name"): must terminate the route.
		if idx+len(wildcard) != len(path) {
			panic("catch-all routes are only allowed at the end of the path in '" +
				fullPath + "'")
		}
		if len(n.path) > 0 && n.path[len(n.path)-1] == '/' {
			panic("catch-all conflicts with existing handle for the path segment root in '" +
				fullPath + "'")
		}

		idx-- // step back onto the '/' preceding the catch-all
		if path[idx] != '/' {
			panic("no / before catch-all in path '" + fullPath + "'")
		}

		// Static prefix up to (but excluding) the '/', branching on '/' into an
		// empty boundary node; the boundary's only child carries the catch-all.
		n.path = path[:idx]
		n.indices = "/"

		boundary := &node{wildChild: true, nType: nodeCatchAll}
		n.children = []*node{boundary}

		catchAll := &node{
			path:      path[idx:],
			nType:     nodeCatchAll,
			paramName: wildcard[1:],
			handlers:  handlers,
			priority:  1,
		}
		boundary.children = []*node{catchAll}
		return
	}

	// No (more) wildcards: the rest of the path is a plain static node.
	n.path = path
	n.handlers = handlers
}

// routeResult carries the outcome of a tree lookup.
type routeResult struct {
	handlers []HandlerFunc
	params   Params
	tsr      bool // trailing-slash redirect recommended
}

// getValue walks the tree to find handlers for path, capturing any URL
// parameters into the returned result. When no handler matches but a trailing
// slash variant would, tsr is set so the caller can issue a redirect.
func (n *node) getValue(path string, params Params) routeResult {
walk:
	for {
		prefix := n.path
		if len(path) > len(prefix) {
			if path[:len(prefix)] != prefix {
				return routeResult{params: params, tsr: tsrForRedirect(n, path)}
			}
			path = path[len(prefix):]

			// If this node has no wildcard child, branch on the next byte.
			if !n.wildChild {
				c := path[0]
				for i := 0; i < len(n.indices); i++ {
					if c == n.indices[i] {
						n = n.children[i]
						continue walk
					}
				}
				tsr := path == "/" && n.handlers != nil
				return routeResult{params: params, tsr: tsr}
			}

			// Follow the wildcard child.
			n = n.children[len(n.children)-1]
			switch n.nType {
			case nodeParam:
				// Capture up to the next '/'.
				end := 0
				for end < len(path) && path[end] != '/' {
					end++
				}

				// A constrained param that rejects the value does not match;
				// the request falls through to 404.
				if n.matcher != nil && !n.matcher(path[:end]) {
					return routeResult{params: params}
				}
				params = append(params, Param{Key: n.paramName, Value: path[:end]})

				if end < len(path) {
					if len(n.children) > 0 {
						path = path[end:]
						n = n.children[0]
						continue walk
					}
					tsr := len(path) == end+1
					return routeResult{params: params, tsr: tsr}
				}

				if n.handlers != nil {
					return routeResult{handlers: n.handlers, params: params}
				}
				// A trailing slash with a registered child would match.
				if len(n.children) == 1 {
					n = n.children[0]
					tsr := (n.path == "/" && n.handlers != nil) ||
						(n.path == "" && n.indices == "/")
					return routeResult{params: params, tsr: tsr}
				}
				return routeResult{params: params}

			case nodeCatchAll:
				params = append(params, Param{Key: n.paramName, Value: path})
				return routeResult{handlers: n.handlers, params: params}

			default:
				return routeResult{params: params}
			}
		}

		if path == prefix {
			// Exact match: return handlers if present.
			if n.handlers != nil {
				return routeResult{handlers: n.handlers, params: params}
			}

			// Recommend a trailing-slash redirect when "path/" exists.
			if path == "/" && n.wildChild && n.nType != nodeRoot {
				return routeResult{params: params, tsr: true}
			}
			for i := 0; i < len(n.indices); i++ {
				if n.indices[i] == '/' {
					n = n.children[i]
					tsr := (len(n.path) == 1 && n.handlers != nil) ||
						(n.nType == nodeCatchAll && n.children[0].handlers != nil)
					return routeResult{params: params, tsr: tsr}
				}
			}
			return routeResult{params: params}
		}

		// path is shorter than prefix: only a trailing-slash redirect can help.
		tsr := path == "/" ||
			(len(prefix) == len(path)+1 && prefix[len(path)] == '/' &&
				path == prefix[:len(path)] && n.handlers != nil)
		return routeResult{params: params, tsr: tsr}
	}
}

// tsrForRedirect reports whether redirecting path to a trailing-slash variant
// (or removing one) would reach a registered handler.
func tsrForRedirect(n *node, path string) bool {
	return len(path)+1 == len(n.path) &&
		n.path[len(path)] == '/' &&
		path == n.path[:len(path)] &&
		n.handlers != nil
}

// incrementChildPriority raises the priority of the child at pos and reorders
// siblings so that higher-priority (more-used) children are matched first.
// It returns the new index of the child that was incremented.
func (n *node) incrementChildPriority(pos int) int {
	cs := n.children
	cs[pos].priority++
	prio := cs[pos].priority

	newPos := pos
	for ; newPos > 0 && cs[newPos-1].priority < prio; newPos-- {
		cs[newPos-1], cs[newPos] = cs[newPos], cs[newPos-1]
	}

	if newPos != pos {
		n.indices = n.indices[:newPos] +
			n.indices[pos:pos+1] +
			n.indices[newPos:pos] +
			n.indices[pos+1:]
	}
	return newPos
}

// findWildcard returns the first wildcard segment in path (including its ':' or
// '*' marker), its start index, and whether the segment is well formed. It
// returns idx == -1 when path contains no wildcard.
func findWildcard(path string) (wildcard string, idx int, valid bool) {
	for start, c := range []byte(path) {
		if c != ':' && c != '*' {
			continue
		}
		valid = true
		for end, ch := range []byte(path[start+1:]) {
			switch ch {
			case '/':
				return path[start : start+1+end], start, valid
			case ':', '*':
				valid = false
			}
		}
		return path[start:], start, valid
	}
	return "", -1, false
}

// longestCommonPrefix returns the length of the longest common prefix of a
// and b.
func longestCommonPrefix(a, b string) int {
	limit := min(len(a), len(b))
	i := 0
	for i < limit && a[i] == b[i] {
		i++
	}
	return i
}
