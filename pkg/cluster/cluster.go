package cluster

import (
	"errors"
	"strings"

	"github.com/cespare/xxhash/v2"
)

// Cluster is a simple static cluster using rendezvous hashing for owner selection.
type Cluster struct {
	nodes    []string
	nodeSet  map[string]struct{}
	self     string
	draining map[string]bool
}

// NewFromCSV creates a Cluster from a comma-separated list of node names.
// nodeList entries should be simple hostnames that resolve to the rtsper instances
// on the Docker network (e.g., "rtsper1,rtsper2").
func NewFromCSV(nodeList string, self string) (*Cluster, error) {
	c := &Cluster{nodeSet: make(map[string]struct{}), draining: make(map[string]bool)}
	if nodeList == "" {
		return nil, errors.New("empty cluster nodes")
	}
	parts := strings.Split(nodeList, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		c.nodes = append(c.nodes, p)
		c.nodeSet[p] = struct{}{}
	}
	if len(c.nodes) == 0 {
		return nil, errors.New("no valid cluster nodes")
	}
	if self == "" {
		self = c.nodes[0]
	}
	if _, ok := c.nodeSet[self]; !ok {
		// allow self not in list but set it anyway
		c.self = self
	} else {
		c.self = self
	}
	return c, nil
}

// Members returns the configured nodes in iteration order.
func (c *Cluster) Members() []string { return c.nodes }

// Owner returns the node name that should own the topic using rendezvous hashing.
// Draining nodes are skipped.
func (c *Cluster) Owner(topic string) string {
	var best string
	var bestScore uint64
	for _, n := range c.nodes {
		if c.draining[n] {
			continue
		}
		key := n + "|" + topic
		score := xxhash.Sum64String(key)
		if best == "" || score > bestScore {
			best = n
			bestScore = score
		}
	}
	return best
}

// IsSelf returns true if the provided node matches this node's identity.
func (c *Cluster) IsSelf(node string) bool {
	return node == c.self
}

// Self returns this node's configured name.
func (c *Cluster) Self() string { return c.self }

// SetDraining marks a node as draining (ignored for new owner selection when true).
func (c *Cluster) SetDraining(node string, d bool) {
	if _, ok := c.nodeSet[node]; ok {
		c.draining[node] = d
	}
}
