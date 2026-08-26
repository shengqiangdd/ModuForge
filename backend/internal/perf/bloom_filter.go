package perf

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// BloomFilter 布隆过滤器 - 用于快速判断元素是否可能存在
type BloomFilter struct {
	bits    []uint64
	numHash int
	size    uint64
}

// NewBloomFilter 创建布隆过滤器
// expectedItems: 预期元素数量
// falsePositiveRate: 期望的假阳性率（如 0.01 = 1%）
func NewBloomFilter(expectedItems int, falsePositiveRate float64) *BloomFilter {
	// 计算最优位数: m = -(n * ln(p)) / (ln(2)^2)
	size := optimalSize(expectedItems, falsePositiveRate)
	numHash := optimalHashes(size, expectedItems)

	return &BloomFilter{
		bits:    make([]uint64, (size+63)/64),
		numHash: numHash,
		size:    size,
	}
}

// optimalSize 计算最优位数组大小
func optimalSize(n int, p float64) uint64 {
	m := -float64(n) * ln(p) / (ln(2) * ln(2))
	return uint64(m) + 1
}

// optimalHashes 计算最优哈希函数数量
func optimalHashes(m uint64, n int) int {
	h := float64(m) / float64(n) * ln(2)
	if h < 1 {
		return 1
	}
	return int(h)
}

// ln 自然对数（使用math包）
func ln(x float64) float64 {
	return math.Log(x)
}

// Add 添加元素
func (f *BloomFilter) Add(item []byte) {
	for i := 0; i < f.numHash; i++ {
		pos := f.hash(item, i)
		idx := pos / 64
		bit := pos % 64
		f.bits[idx] |= 1 << bit
	}
}

// AddString 添加字符串元素
func (f *BloomFilter) AddString(item string) {
	f.Add([]byte(item))
}

// Contains 检查元素是否可能存在
func (f *BloomFilter) Contains(item []byte) bool {
	for i := 0; i < f.numHash; i++ {
		pos := f.hash(item, i)
		idx := pos / 64
		bit := pos % 64
		if f.bits[idx]&(1<<bit) == 0 {
			return false
		}
	}
	return true
}

// ContainsString 检查字符串元素是否可能存在
func (f *BloomFilter) ContainsString(item string) bool {
	return f.Contains([]byte(item))
}

// hash 双重哈希: h(i) = h1 + i * h2
func (f *BloomFilter) hash(item []byte, i int) uint64 {
	h := sha256.Sum256(item)
	h1 := binary.BigEndian.Uint64(h[:8])
	h2 := binary.BigEndian.Uint64(h[8:16])
	return (h1 + uint64(i)*h2) % f.size
}

// ApproximateCount 近似计数（通过填充率估算）
func (f *BloomFilter) ApproximateCount() float64 {
	ones := 0
	for _, word := range f.bits {
		for bit := uint64(0); bit < 64; bit++ {
			if word&(1<<bit) != 0 {
				ones++
			}
		}
	}
	if ones == 0 {
		return 0
	}
	ratio := float64(ones) / float64(f.size)
	if ratio >= 1 {
		ratio = 0.999999
	}
	// 使用公式: n = -(m/k) * ln(1 - X/m)
	return -(float64(f.size) / float64(f.numHash)) * ln(1-ratio)
}
