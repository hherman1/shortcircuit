package shortcircuit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"strings"
)

// Parses an html fragment into its node. Excludes <html>, <body>, and <head> tags.
func Parse(h string) (*html.Node, error) {
	n, err := html.ParseFragment(strings.NewReader(h), nil)
	if err != nil {
		return nil, fmt.Errorf("parse fragment: %w", err)
	}
	return n[0].LastChild.LastChild, nil
}

// An api for tracking changes to an html document.
type Node struct {
	// The backing node
	N *html.Node
	// A log of changes
	Cl *Changelog
}

// If this is a document node, returns the Body node within this document, otherwise returns self.
func (n Node) Body() Node {
	if n.N.Type != html.DocumentNode {
		return n
	}
	for _, maybe := range n.children()[0].children() {
		if maybe.N.DataAtom == atom.Body {
			return maybe
		}
	}
	return n
}

// Fetches the subnode with the given id, if it exists.
func (n Node) ById(id string) *Node {
	for _, sn := range n.children() {
		for _, a := range sn.N.Attr {
			if a.Key == "id" && a.Val == id {
				return &sn
			}
		}
	}
	return nil
}

// All the child nodes of this node
func (n Node) children() []Node {
	var c []Node
	for next := n.N.FirstChild; next != nil; next = next.NextSibling {
		c = append(c, Node{
			N:  next,
			Cl: n.Cl,
		})
	}
	return c
}

// Sets the given attribute to the given value.
func (n *Node) setattr(k, v string) {
	c := setattr{
		Key: k,
		Val: v,
	}
	c.Apply(n.N)
	n.Cl.Buffer = append(n.Cl.Buffer, Change{
		IPath:   path(n.N),
		Setattr: &c,
	})
}

// Removes the given attribute from the node
func (n *Node) rmattr(k string) {
	c := rmattr(k)
	c.Apply(n.N)
	n.Cl.Buffer = append(n.Cl.Buffer, Change{
		IPath:  path(n.N),
		Rmattr: &c,
	})
}

// Inserts the given node at the given index. E.g i <= 0 is a prepend, and i >= len(children) is an append.
func (n *Node) Insert(o *html.Node, i int) {
	c := insertNode{
		i: i,
		n: o,
	}
	c.Apply(n.N)
	n.Cl.Buffer = append(n.Cl.Buffer, Change{
		IPath:      path(n.N),
		InsertNode: &c,
	})
}

// Removes the child at the given node.
func (n *Node) Rm(i int) {
	c := rmnode(i)
	c.Apply(n.N)
	n.Cl.Buffer = append(n.Cl.Buffer, Change{
		IPath:      path(n.N),
		Rmnode: &c,
	})
}

// Computes the ipath to the given node from its root parent (parent node without its own parent)
func path(n *html.Node) ipath {
	i := 0
	for next := n.PrevSibling; next != nil; next = next.PrevSibling {
		i++
	}
	var p ipath
	if n.Parent != nil {
		p = path(n.Parent)
	} else {
		// We want to return the path starting from the parent, not starting from the parent's parent, so the path
		// selecting the parent from nil should be excluded.
		return p
	}
	p = append(p, i)
	return p
}


type Changelog struct {
	// Changes that have been applied to the local tree but have not been flushed to a remote buffer yet.
	Buffer []Change
}


// A change to a node at the given path. Only one of the change types will actually be set.
type Change struct {
	IPath   ipath
	Setattr    *setattr
	Rmattr     *rmattr
	InsertNode *insertNode
	Rmnode *rmnode
}

// Mutates the given node with the given change.
func (c Change) apply(n *html.Node) error {
	target := c.IPath.get(n)
	if target == nil {
		return fmt.Errorf("no node at path: %v", c.IPath)
	}
	switch {
	case c.Rmattr != nil:
		c.Rmattr.Apply(target)
	case c.Setattr != nil:
		c.Setattr.Apply(target)
	case c.InsertNode != nil:
		c.InsertNode.Apply(target)
	}
	return nil
}

// A sequence of child indexes indicating a node to select.
type ipath []int

// Traverses the given node along this path. Returns the resultant node, if there is any.
func (p ipath) get(n *html.Node) *html.Node {
	for _, i := range p {
		next := n.FirstChild
		for ; next != nil; next = next.NextSibling {
			if i == 0 {
				break
			}
			i--
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

// Removes the attribute with the given name
type rmattr string

// Removes this attribute from the given node
func (r rmattr) Apply(n *html.Node) {
	bad := -1
	for i, v := range n.Attr {
		if v.Key == string(r) {
			bad = i
			break
		}
	}
	if bad != -1 {
		n.Attr = append(n.Attr[:bad], n.Attr[bad+1:]...)
	}
}

// Sets the given attribute to the given value
type setattr struct {
	Key string
	Val string
}

func (s setattr) Apply(n *html.Node) {
	for i, a := range n.Attr {
		if a.Key != s.Key {
			continue
		}
		n.Attr[i].Val = s.Val
		return
	}
	n.Attr = append(n.Attr, html.Attribute{
		Key:       s.Key,
		Val:       s.Val,
	})
}

// Inserts a node before the given node index in the parent. If i >= len(children), this is an append. If i <= 0, this is
// a prepend. The node to insert will have its parent/sibling pointers updated.
type insertNode struct {
	i int
	n *html.Node
}

// JSON representation of an insertNode
type insertNodeJSON struct {
	Index int
	Html string
}

func (ins *insertNode) UnmarshalJSON(i []byte) error {
	var j insertNodeJSON
	err := json.Unmarshal(i, &j)
	if err != nil {
		return fmt.Errorf("parse insert node json: %w", err)
	}
	n, err := Parse(j.Html)
	if err != nil {
		return fmt.Errorf("parse insert node html: %w", err)
	}
	ins.i = j.Index
	ins.n = n
	return nil
}

func (ins insertNode) MarshalJSON() ([]byte, error) {
	var j insertNodeJSON
	j.Index = ins.i
	var s bytes.Buffer
	err := html.Render(&s, ins.n)
	if err != nil {
		return nil, fmt.Errorf("render insert html: %w", err)
	}
	j.Html = s.String()
	bs, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf("marshal insert json: %w", err)
	}
	return bs, nil
}

func (ins insertNode) Apply(n *html.Node) {
	// Normalize the node to insert
	ins.n.Parent = n
	ins.n.NextSibling = nil
	ins.n.PrevSibling = nil
	next := n.FirstChild
	if next == nil {
		// there are no child nodes, lets make some.
		n.FirstChild = ins.n
		n.LastChild = ins.n
		return
	}
	for ins.i > 0 {
		if next.NextSibling == nil {
			// i > len(n.children), append
			next.NextSibling = ins.n
			ins.n.PrevSibling = next
			return
		}
		next = next.NextSibling
		ins.i--
	}
	prev := next.PrevSibling
	next.PrevSibling = ins.n
	ins.n.NextSibling = next
	ins.n.PrevSibling = prev
	if prev != nil {
		prev.NextSibling = ins.n
	}
}

// Removes the child node at the given index. Does nothing if the index doesn't exist.
type rmnode int

func (r rmnode) Apply(n *html.Node) {
	if r < 0 {
		return
	}
	for next := n.FirstChild; next != nil; next = next.NextSibling {
		if r != 0 {
			r--
			continue
		}
		if next.NextSibling != nil {
			next.NextSibling = next.PrevSibling
		}
		if next.PrevSibling != nil {
			next.PrevSibling.NextSibling = next.NextSibling
		}
		if next.Parent.FirstChild == next {
			next.Parent.FirstChild = next.NextSibling
		}
		if next.Parent.LastChild == next {
			next.Parent.LastChild = next.PrevSibling
		}
		next.Parent = nil
		next.NextSibling = nil
		next.PrevSibling = nil
		return
	}
}
