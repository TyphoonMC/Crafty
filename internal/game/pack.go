package game

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const (
	PackMagic      = "CRPK"
	PackVersion    = 1
	PackBlockSize  = 16
	PackVoxelCount = PackBlockSize * PackBlockSize * PackBlockSize

	// packZipEntry is the single CRPK blob stored inside a zipped pack file.
	packZipEntry = "pack.bin"
)

// BlockFlag bitmask values stored in BlockDef.Flags.
const (
	BlockFlagTransparent uint8 = 1 << 0
	BlockFlagEmissive    uint8 = 1 << 1
	BlockFlagSolid       uint8 = 1 << 2
)

type RGBA struct {
	R, G, B, A uint8
}

type BlockDef struct {
	Name   string
	Flags  uint8
	Voxels [PackVoxelCount]uint8
}

type Pack struct {
	Palette []RGBA
	Blocks  []BlockDef
}

type packHeader struct {
	Magic       [4]byte
	Version     uint16
	BlockSize   uint8
	Flags       uint8
	BlockCount  uint16
	PaletteSize uint16
}

// VoxelIndex maps (x, y, z) within a block to its offset in the voxel array.
// X varies fastest, then Y, then Z.
func VoxelIndex(x, y, z int) int {
	return x + y*PackBlockSize + z*PackBlockSize*PackBlockSize
}

// LoadPack reads a resource pack from disk. It auto-detects zipped containers
// (PK magic) and falls back to raw CRPK for everything else, so the same path
// works whether the pack was compressed or not.
func LoadPack(path string) (*Pack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if looksLikeZip(data) {
		return decodeZippedPack(data)
	}
	return decodePack(bytes.NewReader(data))
}

func looksLikeZip(b []byte) bool {
	return len(b) >= 4 && b[0] == 'P' && b[1] == 'K' && b[2] == 0x03 && b[3] == 0x04
}

func decodeZippedPack(data []byte) (*Pack, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	for _, zf := range zr.File {
		if zf.Name != packZipEntry {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", packZipEntry, err)
		}
		defer rc.Close()
		return decodePack(rc)
	}
	return nil, fmt.Errorf("zip missing %q entry", packZipEntry)
}

func decodePack(r io.Reader) (*Pack, error) {
	var hdr packHeader
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if string(hdr.Magic[:]) != PackMagic {
		return nil, fmt.Errorf("invalid magic %q (expected %q)", hdr.Magic[:], PackMagic)
	}
	if hdr.Version != PackVersion {
		return nil, fmt.Errorf("unsupported pack version %d", hdr.Version)
	}
	if hdr.BlockSize != PackBlockSize {
		return nil, fmt.Errorf("unsupported block size %d (expected %d)", hdr.BlockSize, PackBlockSize)
	}

	pack := &Pack{
		Palette: make([]RGBA, hdr.PaletteSize),
		Blocks:  make([]BlockDef, hdr.BlockCount),
	}
	if hdr.PaletteSize > 0 {
		if err := binary.Read(r, binary.LittleEndian, pack.Palette); err != nil {
			return nil, fmt.Errorf("read palette: %w", err)
		}
	}

	for i := range pack.Blocks {
		bd := &pack.Blocks[i]

		var nameLen uint8
		if err := binary.Read(r, binary.LittleEndian, &nameLen); err != nil {
			return nil, fmt.Errorf("block %d name length: %w", i, err)
		}
		name := make([]byte, nameLen)
		if _, err := io.ReadFull(r, name); err != nil {
			return nil, fmt.Errorf("block %d name: %w", i, err)
		}
		bd.Name = string(name)

		if err := binary.Read(r, binary.LittleEndian, &bd.Flags); err != nil {
			return nil, fmt.Errorf("block %d flags: %w", i, err)
		}
		var dataLen uint32
		if err := binary.Read(r, binary.LittleEndian, &dataLen); err != nil {
			return nil, fmt.Errorf("block %d data length: %w", i, err)
		}
		if dataLen != PackVoxelCount {
			return nil, fmt.Errorf("block %d: expected %d voxel bytes, got %d", i, PackVoxelCount, dataLen)
		}
		if _, err := io.ReadFull(r, bd.Voxels[:]); err != nil {
			return nil, fmt.Errorf("block %d voxels: %w", i, err)
		}
	}
	return pack, nil
}

// WritePack serializes the pack to disk as a raw CRPK blob.
func WritePack(path string, pack *Pack) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return encodePack(f, pack)
}

// WritePackZip serializes the pack inside a zip container (single pack.bin
// entry). Deflate typically shrinks uniform voxel blocks by ~40x.
func WritePackZip(path string, pack *Pack) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	w, err := zw.Create(packZipEntry)
	if err != nil {
		_ = zw.Close()
		return err
	}
	if err := encodePack(w, pack); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func encodePack(w io.Writer, pack *Pack) error {
	if len(pack.Palette) > 0xFFFF {
		return fmt.Errorf("palette too large: %d entries", len(pack.Palette))
	}
	if len(pack.Blocks) > 0xFFFF {
		return fmt.Errorf("too many blocks: %d", len(pack.Blocks))
	}

	hdr := packHeader{
		Version:     PackVersion,
		BlockSize:   PackBlockSize,
		Flags:       0,
		BlockCount:  uint16(len(pack.Blocks)),
		PaletteSize: uint16(len(pack.Palette)),
	}
	copy(hdr.Magic[:], PackMagic)

	if err := binary.Write(w, binary.LittleEndian, hdr); err != nil {
		return err
	}
	if len(pack.Palette) > 0 {
		if err := binary.Write(w, binary.LittleEndian, pack.Palette); err != nil {
			return err
		}
	}

	for i := range pack.Blocks {
		bd := &pack.Blocks[i]
		if len(bd.Name) > 0xFF {
			return fmt.Errorf("block %d name too long", i)
		}
		if err := binary.Write(w, binary.LittleEndian, uint8(len(bd.Name))); err != nil {
			return err
		}
		if _, err := w.Write([]byte(bd.Name)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, bd.Flags); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(PackVoxelCount)); err != nil {
			return err
		}
		if _, err := w.Write(bd.Voxels[:]); err != nil {
			return err
		}
	}
	return nil
}
