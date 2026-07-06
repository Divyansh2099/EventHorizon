package router

import (
	"bytes"
	"sync/atomic"
	"unsafe"
	"github.com/eventhorizon/pkg/parser"
)

type HandlerFunc func(ctx *parser.RequestCtx)

type node struct {
	segment  []byte
	param    string
	isParam  bool
	handler  HandlerFunc
	children []*node
}

func (n *node) insert(segments [][]byte, handler HandlerFunc) {
	if len(segments) == 0 {
		n.handler = handler
		return
	}

	segment := segments[0]
	isParam := false
	paramKey := ""

	if len(segment) > 0 && segment[0] == ':' {
		isParam = true
		paramKey = string(segment[1:])
	}

	var child *node
	for _, c := range n.children {
		if c.isParam == isParam && bytes.Equal(c.segment, segment) {
			child = c
			break
		}
	}

	if child == nil {
		child = &node{
			segment: segment,
			param:   paramKey,
			isParam: isParam,
		}
		n.children = append(n.children, child)
	}

	child.insert(segments[1:], handler)
}

func (n *node) search(path []byte, ctx *parser.RequestCtx, absoluteOffset uint32) HandlerFunc {
	// Root match
	if len(path) == 0 || (len(path) == 1 && path[0] == '/') {
		return n.handler
	}
	
	// Strip leading slash for processing segments
	if path[0] == '/' {
		path = path[1:]
		absoluteOffset++
	}

	slashIdx := bytes.IndexByte(path, '/')
	var segment []byte
	var remaining []byte
	var remainingOffset uint32

	if slashIdx == -1 {
		segment = path
	} else {
		segment = path[:slashIdx]
		remaining = path[slashIdx:]
		remainingOffset = absoluteOffset + uint32(slashIdx)
	}

	for _, child := range n.children {
		if child.isParam {
			// Zero-allocation parameter extraction!
			if ctx != nil && len(segment) > 0 {
				ctx.AddParam(child.param, absoluteOffset, absoluteOffset+uint32(len(segment)))
			}
			if len(remaining) == 0 || (len(remaining) == 1 && remaining[0] == '/') {
				return child.handler
			}
			handler := child.search(remaining, ctx, remainingOffset)
			if handler != nil {
				return handler
			}
		} else if bytes.Equal(child.segment, segment) {
			if len(remaining) == 0 || (len(remaining) == 1 && remaining[0] == '/') {
				return child.handler
			}
			handler := child.search(remaining, ctx, remainingOffset)
			if handler != nil {
				return handler
			}
		}
	}

	return nil
}

type Router struct {
	trees atomic.Value // map[string]*node
	WSRoute func(conn any, frame any)
}

func New() *Router {
	r := &Router{}
	r.trees.Store(make(map[string]*node))
	return r
}

func splitPath(path string) [][]byte {
	if path == "/" {
		return nil
	}
	if path[0] == '/' {
		path = path[1:]
	}
	if len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	parts := bytes.Split([]byte(path), []byte("/"))
	return parts
}

func (r *Router) Handle(method, path string, handler HandlerFunc) {
	oldTrees := r.trees.Load().(map[string]*node)
	newTrees := make(map[string]*node)

	// Copy existing trees (we only rebuild at startup, so this is fine)
	for m, root := range oldTrees {
		newTrees[m] = root // In a fully dynamic router we'd deep copy, but routes are static after init
	}

	root := newTrees[method]
	if root == nil {
		root = &node{}
		newTrees[method] = root
	}

	segments := splitPath(path)
	root.insert(segments, handler)

	r.trees.Store(newTrees)
}

// Lookup finds the handler and extracts zero-allocation parameters into the RequestCtx.
// It requires the method, the path bytes, and the RequestCtx (to calculate absolute boundaries).
func (r *Router) Lookup(method []byte, path []byte, ctx *parser.RequestCtx) HandlerFunc {
	trees := r.trees.Load().(map[string]*node)
	
	// Zero-allocation string conversion for map lookup of methods (GET, POST)
	m := unsafeString(method)
	root := trees[m]
	
	if root == nil {
		return nil
	}

	// Calculate the absolute offset of the path within the connection's ReadBuffer.
	// This ensures our parameter spans perfectly align with the hardware-pinned memory.
	var absoluteOffset uint32
	if ctx != nil {
		absoluteOffset = ctx.Path.Start
	}

	return root.search(path, ctx, absoluteOffset)
}

// unsafeString provides a zero-allocation string cast for map lookups.
func unsafeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}
