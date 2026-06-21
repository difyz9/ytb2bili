package store

import (
	"container/heap"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// CacheItem 表示字典中的一项数据
type CacheItem struct {
	Key      string      // 键
	Value    interface{} // 值
	ExpireAt time.Time   // 过期时间
	index    int         // 在堆中的索引
}

// CacheDict 临时字典结构
type CacheDict struct {
	mu    sync.Mutex            // 互斥锁
	items map[string]*CacheItem // 数据存储
	queue PriorityQueue         // 优先队列（最小堆），用于快速获取最早过期的项
	stop  chan struct{}         // 停止信号
}

// PriorityQueue 实现heap.Interface接口的优先队列
type PriorityQueue []*CacheItem

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].ExpireAt.Before(pq[j].ExpireAt)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*CacheItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // 标记为已移除
	*pq = old[0 : n-1]
	return item
}

// NewTempDict 创建一个新的临时字典
func NewTempDict() *CacheDict {
	td := &CacheDict{
		items: make(map[string]*CacheItem),
		queue: make(PriorityQueue, 0),
		stop:  make(chan struct{}),
	}
	heap.Init(&td.queue)
	go td.cleanupWorker()
	return td
}

// Set 添加或更新一个项，并设置过期时间
func (td *CacheDict) Set(key string, value interface{}, ttl time.Duration) {
	td.mu.Lock()
	defer td.mu.Unlock()

	expireAt := time.Now().Add(ttl)

	// 如果键已存在，先移除旧项
	if item, exists := td.items[key]; exists {
		heap.Remove(&td.queue, item.index)
	}

	// 创建新项
	item := &CacheItem{
		Key:      key,
		Value:    value,
		ExpireAt: expireAt,
	}

	td.items[key] = item
	heap.Push(&td.queue, item)
}

// SetWithDefaultTTL 使用默认5分钟过期时间添加或更新一个项
func (td *CacheDict) SetWithDefaultTTL(key string, value interface{}) {
	td.Set(key, value, 5*time.Minute)
}

// Get 获取一个项的值，如果不存在或已过期则返回错误
func (td *CacheDict) Get(key string, dest interface{}) error {
	td.mu.Lock()
	defer td.mu.Unlock()

	item, exists := td.items[key]
	if !exists {
		return errors.New("key not found")
	}

	if time.Now().After(item.ExpireAt) {
		heap.Remove(&td.queue, item.index)
		delete(td.items, key)
		return errors.New("key expired")
	}

	// 使用JSON编解码来复制值到目标对象
	data, err := json.Marshal(item.Value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Delete 删除一个项
func (td *CacheDict) Delete(key string) {
	td.mu.Lock()
	defer td.mu.Unlock()

	if item, exists := td.items[key]; exists {
		heap.Remove(&td.queue, item.index)
		delete(td.items, key)
	}
}

// cleanupWorker 后台清理过期项的goroutine
func (td *CacheDict) cleanupWorker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			td.cleanupExpired()
		case <-td.stop:
			return
		}
	}
}

// cleanupExpired 清理所有过期的项
func (td *CacheDict) cleanupExpired() {
	td.mu.Lock()
	defer td.mu.Unlock()

	now := time.Now()
	for td.queue.Len() > 0 {
		item := td.queue[0] // 查看最早过期的项
		if now.Before(item.ExpireAt) {
			break // 没有过期项了
		}

		heap.Pop(&td.queue)
		delete(td.items, item.Key)
	}
}

// Close 停止临时字典的后台清理工作
func (td *CacheDict) Close() {
	close(td.stop)
}

// Size 返回当前字典中的项数
func (td *CacheDict) Size() int {
	td.mu.Lock()
	defer td.mu.Unlock()
	return len(td.items)
}

// Has 检查键是否存在且未过期
func (td *CacheDict) Has(key string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()

	item, exists := td.items[key]
	if !exists {
		return false
	}

	if time.Now().After(item.ExpireAt) {
		heap.Remove(&td.queue, item.index)
		delete(td.items, key)
		return false
	}

	return true
}

// Keys 返回所有未过期的键
func (td *CacheDict) Keys() []string {
	td.mu.Lock()
	defer td.mu.Unlock()

	now := time.Now()
	keys := make([]string, 0, len(td.items))

	for key, item := range td.items {
		if now.Before(item.ExpireAt) {
			keys = append(keys, key)
		}
	}

	return keys
}
