// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binres

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The encodings exercised here are what a modern aapt2 emits and the
// gomobile-era decoder could not read (#287). They are unreachable from the
// embedded table — arsc.go is written by this package's own marshaler, so it
// is always dense/legacy — and the tests that read a real platform table are
// skipped on any SDK past android-15 (#297). Synthesizing the chunks is
// therefore the only coverage these paths get.

// typeChunk assembles a ResTable_type chunk carrying the given entry index and
// entry payload. It mirrors the 56-byte header Type.MarshalBinary writes.
func typeChunk(id, flags uint8, entryCount uint32, index, entries []byte) []byte {
	const headerSize = 56

	idx := append([]byte(nil), index...)
	for len(idx)%4 != 0 { // entry data is 4-byte aligned
		idx = append(idx, 0)
	}

	bin := make([]byte, headerSize)
	putu16(bin, uint16(ResTableType))
	putu16(bin[2:], headerSize)
	bin[8] = id
	bin[9] = flags
	putu32(bin[12:], entryCount)
	putu32(bin[16:], uint32(headerSize+len(idx))) // entriesStart
	putu32(bin[20:], 36)                          // config size

	bin = append(bin, idx...)
	bin = append(bin, entries...)
	putu32(bin[4:], uint32(len(bin)))
	return bin
}

// simpleEntry returns the classic 16-byte entry: an 8-byte ResTable_entry
// followed by an 8-byte Res_value.
func simpleEntry(key uint32, typ DataType, value uint32) []byte {
	bin := make([]byte, 16)
	putu16(bin, 8) // size
	// [2:4] flags = 0
	putu32(bin[4:], key)
	putu16(bin[8:], 8) // value byte size
	// [10] res0 = 0
	bin[11] = byte(typ)
	putu32(bin[12:], value)
	return bin
}

// compactEntry returns the 8-byte compact entry aapt2 emits since API 35:
// {u16 key; u16 flags} with the data type in the flags high byte, then u32 data.
func compactEntry(key uint16, typ DataType, value uint32) []byte {
	bin := make([]byte, 8)
	putu16(bin, key)
	putu16(bin[2:], uint16(typ)<<8|entryFlagCompact)
	putu32(bin[4:], value)
	return bin
}

func TestTypeUnmarshalSparseIndex(t *testing.T) {
	r := require.New(t)

	// entry ids 2 and 5 only; offsets are in units of 4 bytes.
	index := make([]byte, 8)
	putu16(index, 2)
	putu16(index[2:], 0)
	putu16(index[4:], 5)
	putu16(index[6:], 16/4)

	entries := append(simpleEntry(7, DataIntDec, 0x11), simpleEntry(9, DataIntHex, 0x2000)...)

	typ := new(Type)
	r.NoError(typ.UnmarshalBinary(typeChunk(1, typeFlagSparse, 2, index, entries)))

	// normalized to the dense legacy layout MarshalBinary emits
	r.Equal(uint8(0), typ.flags)
	r.Equal([]uint32{NoEntry, NoEntry, 0, NoEntry, NoEntry, 16}, typ.indices)
	r.Len(typ.entries, 6)
	r.Equal(uint32(6), typ.entryCount)

	for _, i := range []int{0, 1, 3, 4} {
		r.Nil(typ.entries[i], "entry %d should be absent", i)
	}
	r.Equal(PoolRef(7), typ.entries[2].key)
	r.Equal(uint32(0x11), typ.entries[2].values[0].data.Value)
	r.Equal(PoolRef(9), typ.entries[5].key)
	r.Equal(uint32(0x2000), typ.entries[5].values[0].data.Value)
}

func TestTypeUnmarshalOffset16Index(t *testing.T) {
	r := require.New(t)

	// offsets in units of 4 bytes; 0xFFFF marks a hole.
	index := make([]byte, 8)
	putu16(index, 0)
	putu16(index[2:], 0xFFFF)
	putu16(index[4:], 16/4)
	putu16(index[6:], 0xFFFF)

	entries := append(simpleEntry(3, DataIntHex, 0x1000), simpleEntry(4, DataIntHex, 0x2000)...)

	typ := new(Type)
	r.NoError(typ.UnmarshalBinary(typeChunk(1, typeFlagOffset16, 4, index, entries)))

	r.Equal(uint8(0), typ.flags)
	r.Equal([]uint32{0, NoEntry, 16, NoEntry}, typ.indices)
	r.Len(typ.entries, 4)
	r.Equal(uint32(4), typ.entryCount)
	r.Nil(typ.entries[1])
	r.Nil(typ.entries[3])
	r.Equal(PoolRef(3), typ.entries[0].key)
	r.Equal(uint32(0x1000), typ.entries[0].values[0].data.Value)
	r.Equal(PoolRef(4), typ.entries[2].key)
	r.Equal(uint32(0x2000), typ.entries[2].values[0].data.Value)
}

func TestTypeUnmarshalRejectsUnknownFlags(t *testing.T) {
	r := require.New(t)

	err := new(Type).UnmarshalBinary(typeChunk(1, 0x80, 0, nil, nil))
	r.Error(err)
	r.Contains(err.Error(), "flags")
}

func TestTypeUnmarshalTruncatedIndex(t *testing.T) {
	r := require.New(t)

	// entryCount claims two sparse pairs but only one is present.
	index := make([]byte, 4)
	putu16(index, 1)
	putu16(index[2:], 0)

	err := new(Type).UnmarshalBinary(typeChunk(1, typeFlagSparse, 2, index, simpleEntry(1, DataIntDec, 1)))
	r.Error(err)
	r.Contains(err.Error(), "sparse index too short")
}

func TestEntryUnmarshalCompact(t *testing.T) {
	r := require.New(t)

	nt := new(Entry)
	r.NoError(nt.UnmarshalBinary(compactEntry(42, DataIntHex, 0x22a4)))

	// normalized to the classic simple entry MarshalBinary emits
	r.Equal(uint16(8), nt.size)
	r.Equal(uint16(0), nt.flags&entryFlagCompact)
	r.Equal(uint16(0), nt.flags>>8, "data type must not be left in the flags high byte")
	r.Equal(PoolRef(42), nt.key)
	r.Len(nt.values, 1)
	r.Equal(DataIntHex, nt.values[0].data.Type)
	r.Equal(uint32(0x22a4), nt.values[0].data.Value)
}

func TestTypeUnmarshalCompactEntries(t *testing.T) {
	r := require.New(t)

	// the android-35 combination: an offset16 index over compact entries.
	index := make([]byte, 4)
	putu16(index, 0)
	putu16(index[2:], 8/4)

	entries := append(compactEntry(1, DataIntHex, 0x2000), compactEntry(2, DataIntBool, 0xFFFFFFFF)...)

	typ := new(Type)
	r.NoError(typ.UnmarshalBinary(typeChunk(1, typeFlagOffset16, 2, index, entries)))

	r.Len(typ.entries, 2)
	r.Equal(PoolRef(1), typ.entries[0].key)
	r.Equal(uint32(0x2000), typ.entries[0].values[0].data.Value)
	r.Equal(PoolRef(2), typ.entries[1].key)
	r.Equal(DataIntBool, typ.entries[1].values[0].data.Type)
}

// metadataChunk returns an otherwise empty chunk of type t, as aapt2 emits for
// library / overlayable / staged-alias metadata inside a package.
func metadataChunk(t ResType, size int) []byte {
	bin := make([]byte, size)
	putu16(bin, uint16(t))
	putu16(bin[2:], 8)
	putu32(bin[4:], uint32(size))
	return bin
}

func TestPackageUnmarshalSkipsMetadataChunks(t *testing.T) {
	for _, mt := range []ResType{ResTableLibrary, ResTableOverlayable, ResTableOverlayablePolicy, ResTableStagedAlias} {
		t.Run(mt.String(), func(t *testing.T) {
			r := require.New(t)

			tbl, _ := NewMipmapTable("com.example.metadata")
			bin, err := tbl.pkgs[0].MarshalBinary()
			r.NoError(err)

			bin = append(bin, metadataChunk(mt, 16)...)
			putu32(bin[4:], uint32(len(bin))) // grow the package chunk over it

			pkg := new(Package)
			r.NoError(pkg.UnmarshalBinary(bin))
			r.Len(pkg.specs, 2)
			r.Equal([]string{"attr", "mipmap"}, pkg.typePool.strings)
		})
	}
}

func TestPackageUnmarshalRejectsZeroSizedMetadataChunk(t *testing.T) {
	r := require.New(t)

	tbl, _ := NewMipmapTable("com.example.metadata")
	bin, err := tbl.pkgs[0].MarshalBinary()
	r.NoError(err)

	// a zero size would leave the package walk spinning on the same chunk
	bin = append(bin, metadataChunk(ResTableStagedAlias, 16)...)
	putu32(bin[len(bin)-16+4:], 0)
	putu32(bin[4:], uint32(len(bin)))

	err = new(Package).UnmarshalBinary(bin)
	r.Error(err)
	r.Contains(err.Error(), "out of range")
}

// TestTableUnmarshalMultiPackage locks the package walk to its own chunk: a
// second package used to be read as trailing garbage inside the first.
func TestTableUnmarshalMultiPackage(t *testing.T) {
	r := require.New(t)

	first, _ := NewMipmapTable("com.example.one")
	second, _ := NewMipmapTable("com.example.two")
	second.pkgs[0].id = 128
	first.pkgs = append(first.pkgs, second.pkgs[0])

	bin, err := first.MarshalBinary()
	r.NoError(err)

	tbl := new(Table)
	r.NoError(tbl.UnmarshalBinary(bin))
	r.Len(tbl.pkgs, 2)
	r.Equal("com.example.one", tbl.pkgs[0].name)
	r.Equal("com.example.two", tbl.pkgs[1].name)
	for _, pkg := range tbl.pkgs {
		r.Len(pkg.specs, 2)
	}
}
