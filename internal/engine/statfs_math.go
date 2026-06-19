package engine

const (
	maxInt     = int(^uint(0) >> 1)
	maxIntUint = uint64(^uint(0) >> 1)
)

func statfsBlockBytes(blocks, blockSize uint64) int {
	if blocks == 0 || blockSize == 0 {
		return 0
	}
	if blockSize > maxIntUint || blocks > maxIntUint/blockSize {
		return maxInt
	}
	return int(blocks * blockSize) // #nosec G115 -- product is bounded by maxIntUint before conversion.
}

func statfsNativeBlockBytes[T ~int64 | ~uint64](blocks T, blockSize uint64) int {
	if blocks <= 0 {
		return 0
	}
	return statfsBlockBytes(uint64(blocks), blockSize)
}

func statfsNativeUsedBytes[T ~int64 | ~uint64](blocks, bfree T, blockSize uint64) int {
	if blocks <= 0 || bfree < 0 {
		return 0
	}
	return statfsUsedBlockBytes(uint64(blocks), uint64(bfree), blockSize)
}

func statfsUsedBlockBytes(blocks, bfree, blockSize uint64) int {
	if bfree >= blocks {
		return 0
	}
	return statfsBlockBytes(blocks-bfree, blockSize)
}
