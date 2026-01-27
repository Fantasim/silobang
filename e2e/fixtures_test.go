package e2e

import (
	"crypto/rand"
	"encoding/binary"
)

// GenerateTestFile creates a random file of given size
func GenerateTestFile(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

// GenerateTestGLB creates a minimal valid GLB file
// GLB format: https://registry.khronos.org/glTF/specs/2.0/glTF-2.0.html#glb-file-format-specification
func GenerateTestGLB(size int) []byte {
	if size < 12 {
		size = 12 // Minimum GLB header size
	}

	data := make([]byte, size)

	// Magic: "glTF" (0x46546C67 in little-endian)
	data[0] = 0x67
	data[1] = 0x6C
	data[2] = 0x54
	data[3] = 0x46

	// Version: 2 (uint32 little-endian)
	binary.LittleEndian.PutUint32(data[4:8], 2)

	// Length: total file size (uint32 little-endian)
	binary.LittleEndian.PutUint32(data[8:12], uint32(size))

	// Fill rest with random data
	if size > 12 {
		rand.Read(data[12:])
	}

	return data
}

// Pre-generated test files for common scenarios
var (
	SmallFile  = GenerateTestFile(1024)        // 1 KB
	MediumFile = GenerateTestFile(100 * 1024)  // 100 KB
	LargeFile  = GenerateTestFile(1024 * 1024) // 1 MB
)
