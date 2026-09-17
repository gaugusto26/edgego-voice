package audiocache

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"sync"
	"time"
)

const (
	numShards = 16 // 16 shards independentes para minimizar contenção de lock concorrente
)

// bufferPool reutiliza buffers para evitar alocações de heap na geração de chaves (golang-performance).
var bufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 256)
		return bytes.NewBuffer(b)
	},
}

type segmentType int

const (
	segmentProbationary segmentType = iota // Primeiro acesso: 25% da cota
	segmentProtected                       // Múltiplos acessos (frequente): 75% da cota
)

type cacheItem struct {
	key       string
	data      []byte
	size      int64
	hits      int32
	segment   segmentType
	lastTouch time.Time
	expiresAt time.Time
}

// cacheShard representa um shard isolado com Segmented LRU (SLRU).
type cacheShard struct {
	mu             sync.RWMutex
	items          map[string]*list.Element
	probationList  *list.List // Fila probatória para itens recentes
	protectedList  *list.List // Fila protegida para itens frequentes (imune a scan pollution)
	maxBytes       int64
	curBytes       int64
	probationBytes int64
	protectedBytes int64
	ttl            time.Duration
}

func newCacheShard(maxBytes int64, ttl time.Duration) *cacheShard {
	return &cacheShard{
		items:         make(map[string]*list.Element),
		probationList: list.New(),
		protectedList: list.New(),
		maxBytes:      maxBytes,
		ttl:           ttl,
	}
}

// Cache é um cache em memória Segmented-LRU (SLRU) de alta performance, sharded e thread-safe.
// Projetado especificamente para baixa latência (<0.2ms) e proteção de prompts frequentes em telefonia.
type Cache struct {
	shards   [numShards]*cacheShard
	maxBytes int64
	ttl      time.Duration
	enabled  bool
}

// NewCache cria uma nova instância de cache particionado (sharded) de áudio.
func NewCache(enabled bool, maxMB int, ttl time.Duration) *Cache {
	totalBytes := int64(maxMB) * 1024 * 1024
	bytesPerShard := totalBytes / numShards
	if bytesPerShard <= 0 {
		bytesPerShard = 1024 * 1024 // fallback mínimo de 1MB por shard
	}

	c := &Cache{
		maxBytes: totalBytes,
		ttl:      ttl,
		enabled:  enabled,
	}

	for i := 0; i < numShards; i++ {
		c.shards[i] = newCacheShard(bytesPerShard, ttl)
	}

	return c
}

// fnv32 calcula o hash FNV-1a de 32 bits para distribuição uniforme e rápida entre os shards.
func fnv32(key string) uint32 {
	var hash uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return hash
}

func (c *Cache) getShard(key string) *cacheShard {
	idx := fnv32(key) % numShards
	return c.shards[idx]
}

// GenerateKey gera um hash SHA256 único a partir dos parâmetros de síntese.
func GenerateKey(text, voice, format string, speed float64, pitch string, removeFilter bool) string {
	return GenerateKeyWithOptions(text, voice, format, speed, pitch, removeFilter, false, false)
}

// GenerateKeyWithOptions gera um hash SHA256 único incluindo opções avançadas como filtro de telefonia e respiração.
func GenerateKeyWithOptions(text, voice, format string, speed float64, pitch string, removeFilter, telephony, autoBreath bool) string {
	return GenerateKeyWithEmotions(text, voice, format, speed, pitch, removeFilter, telephony, autoBreath, "", "", "")
}

// GenerateKeyWithEmotions gera um hash SHA256 único incluindo opções avançadas, sentimentos e expressividade.
// Otimizado com buffer pool para eliminar alocações de memória na rota crítica (golang-performance).
func GenerateKeyWithEmotions(text, voice, format string, speed float64, pitch string, removeFilter, telephony, autoBreath bool, emotion, emotionScale, style string) string {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	buf.WriteString(text)
	buf.WriteByte('|')
	buf.WriteString(voice)
	buf.WriteByte('|')
	buf.WriteString(format)
	buf.WriteByte('|')
	buf.WriteString(strconv.FormatFloat(speed, 'f', 2, 64))
	buf.WriteByte('|')
	buf.WriteString(pitch)
	buf.WriteByte('|')
	buf.WriteString(strconv.FormatBool(removeFilter))
	if telephony {
		buf.WriteString("|tel:true")
	}
	if autoBreath {
		buf.WriteString("|breath:true")
	}
	if emotion != "" {
		buf.WriteString("|emo:")
		buf.WriteString(emotion)
		buf.WriteString(":")
		buf.WriteString(emotionScale)
	}
	if style != "" {
		buf.WriteString("|style:")
		buf.WriteString(style)
	}

	hash := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(hash[:])
}

// Get busca um áudio no cache em tempo sub-milissegundo (< 0.2ms).
func (c *Cache) Get(key string) ([]byte, bool) {
	if !c.enabled {
		return nil, false
	}

	shard := c.getShard(key)
	now := time.Now()

	shard.mu.Lock()
	defer shard.mu.Unlock()

	elem, found := shard.items[key]
	if !found {
		return nil, false
	}

	item := elem.Value.(*cacheItem)
	if now.After(item.expiresAt) {
		shard.removeElement(elem)
		return nil, false
	}

	item.hits++

	// Se o item já foi acessado há menos de 500ms, dispensamos a reorganização da lista
	// para economizar ciclos de CPU e escrita na memória em requisições consecutivas.
	shouldReorder := now.Sub(item.lastTouch) > 500*time.Millisecond
	item.lastTouch = now

	if shouldReorder {
		// Promoção SLRU: se estava na fila probatória e foi acessado novamente,
		// promove para a fila protegida (prompts e saudações frequentes de telefonia).
		if item.segment == segmentProbationary && item.hits >= 2 {
			shard.probationList.Remove(elem)
			shard.probationBytes -= item.size

			item.segment = segmentProtected
			newElem := shard.protectedList.PushFront(item)
			shard.items[key] = newElem
			shard.protectedBytes += item.size

			// Se a fila protegida estourar a cota (75%), o item menos usado dela volta para probatório
			shard.balanceProtected()
		} else if item.segment == segmentProtected {
			shard.protectedList.MoveToFront(elem)
		} else {
			shard.probationList.MoveToFront(elem)
		}
	}

	return item.data, true
}

// Set armazena um áudio sintetizado no cache com admissão inteligente.
func (c *Cache) Set(key string, data []byte) {
	if !c.enabled || len(data) == 0 {
		return
	}

	itemSize := int64(len(data))
	shard := c.getShard(key)

	if itemSize > shard.maxBytes {
		return // Não armazena arquivos que sozinhos superem a cota do shard
	}

	now := time.Now()

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Se já existe, atualiza os dados e renova o TTL
	if elem, found := shard.items[key]; found {
		oldItem := elem.Value.(*cacheItem)
		diff := itemSize - oldItem.size

		oldItem.data = data
		oldItem.size = itemSize
		oldItem.expiresAt = now.Add(shard.ttl)
		oldItem.lastTouch = now

		shard.curBytes += diff
		if oldItem.segment == segmentProtected {
			shard.protectedBytes += diff
			shard.protectedList.MoveToFront(elem)
		} else {
			shard.probationBytes += diff
			shard.probationList.MoveToFront(elem)
		}

		shard.evictIfNeeded()
		return
	}

	// Novo item: entra inicialmente na fila probatória (SLRU)
	item := &cacheItem{
		key:       key,
		data:      data,
		size:      itemSize,
		hits:      1,
		segment:   segmentProbationary,
		lastTouch: now,
		expiresAt: now.Add(shard.ttl),
	}

	elem := shard.probationList.PushFront(item)
	shard.items[key] = elem
	shard.curBytes += itemSize
	shard.probationBytes += itemSize

	shard.evictIfNeeded()
}

// balanceProtected mantém a fila protegida dentro de 75% da capacidade máxima do shard.
// O item excedente é rebaixado de volta à fila probatória em vez de ser descartado da memória.
func (s *cacheShard) balanceProtected() {
	maxProtected := (s.maxBytes * 3) / 4 // 75%
	for s.protectedBytes > maxProtected && s.protectedList.Len() > 0 {
		demoteElem := s.protectedList.Back()
		if demoteElem == nil {
			break
		}
		item := demoteElem.Value.(*cacheItem)
		s.protectedList.Remove(demoteElem)
		s.protectedBytes -= item.size

		item.segment = segmentProbationary
		newElem := s.probationList.PushFront(item)
		s.items[item.key] = newElem
		s.probationBytes += item.size
	}
}

// evictIfNeeded descarta primeiro os itens da fila probatória (preservando os itens frequentes).
func (s *cacheShard) evictIfNeeded() {
	for s.curBytes > s.maxBytes {
		// 1. Tenta descartar o item mais antigo da fila probatória
		if s.probationList.Len() > 0 {
			oldest := s.probationList.Back()
			if oldest != nil {
				s.removeElement(oldest)
				continue
			}
		}

		// 2. Se a fila probatória estiver vazia, descarta o item mais antigo da fila protegida
		if s.protectedList.Len() > 0 {
			oldest := s.protectedList.Back()
			if oldest != nil {
				s.removeElement(oldest)
				continue
			}
		}

		break
	}
}

// removeElement remove um item da lista correspondente e do mapa.
func (s *cacheShard) removeElement(elem *list.Element) {
	item := elem.Value.(*cacheItem)
	if item.segment == segmentProtected {
		s.protectedList.Remove(elem)
		s.protectedBytes -= item.size
	} else {
		s.probationList.Remove(elem)
		s.probationBytes -= item.size
	}

	delete(s.items, item.key)
	s.curBytes -= item.size
}

// Stats retorna estatísticas globais somando todos os 16 shards do cache.
func (c *Cache) Stats() (int, int64, int64) {
	totalItems := 0
	var totalBytes int64 = 0

	for i := 0; i < numShards; i++ {
		shard := c.shards[i]
		shard.mu.RLock()
		totalItems += len(shard.items)
		totalBytes += shard.curBytes
		shard.mu.RUnlock()
	}

	return totalItems, totalBytes, c.maxBytes
}
