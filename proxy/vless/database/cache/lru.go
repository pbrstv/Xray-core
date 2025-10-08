package cache

type LRUNode struct {
	key  string
	prev *LRUNode
	next *LRUNode
}

type LRUManager struct {
	head    *LRUNode
	tail    *LRUNode
	nodeMap map[string]*LRUNode
}

func NewLRUManager() *LRUManager {
	return &LRUManager{
		nodeMap: make(map[string]*LRUNode),
	}
}

// Add adds a new node to the beginning of the LRU list
func (lru *LRUManager) Add(key string) *LRUNode {
	node := &LRUNode{key: key}

	if lru.head == nil {
		lru.head = node
		lru.tail = node
	} else {
		node.next = lru.head
		lru.head.prev = node
		lru.head = node
	}

	lru.nodeMap[key] = node
	return node
}

// MoveToFront moves an existing node to the beginning of the LRU list
func (lru *LRUManager) MoveToFront(node *LRUNode) {
	if lru.head == node {
		return // Already at the beginning
	}

	lru.Remove(node)

	// Add to the beginning
	node.prev = nil
	node.next = lru.head
	if lru.head != nil {
		lru.head.prev = node
	}
	lru.head = node
}

func (lru *LRUManager) Remove(node *LRUNode) {
	if node == nil {
		return
	}

	if node == lru.head {
		lru.head = node.next
	}
	if node == lru.tail {
		lru.tail = node.prev
	}

	if node.prev != nil {
		node.prev.next = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	}

	// Remove from hash table
	delete(lru.nodeMap, node.key)
}

// RemoveTail removes the last node from the LRU list
func (lru *LRUManager) RemoveTail() {
	if lru.tail != nil {
		lru.Remove(lru.tail)
	}
}

// GetNode returns a node by key
func (lru *LRUManager) GetNode(key string) (*LRUNode, bool) {
	node, exists := lru.nodeMap[key]
	return node, exists
}
