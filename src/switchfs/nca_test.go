package switchfs

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/trembon/switch-library-manager/settings"
	"github.com/trembon/switch-library-manager/switchfs/_crypto"
)

const (
	testHeaderKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	testAreaKey   = "1f1e1d1c1b1a19181716151413121110"
	testNcaKey    = "00112233445566778899aabbccddeeff"
)

func TestNintendoTweak(t *testing.T) {
	if got := getNintendoTweak(0x010203); got != [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3} {
		t.Fatalf("unexpected Nintendo tweak: %x", got)
	}
}

func TestNcaHeaderAndFsValidation(t *testing.T) {
	if _, err := DecryptNcaHeader("not-hex", make([]byte, 0x400)); err == nil {
		t.Fatal("expected invalid key error")
	}
	if _, err := DecryptNcaHeader("000102030405060708090a0b0c0d0e0f", make([]byte, 0x3ff)); err == nil {
		t.Fatal("expected short header error")
	}
	if _, err := _decryptNcaHeader(nil, make([]byte, 0x10), 0x10, 0x10, 0); err == nil {
		t.Fatal("expected invalid decryption bounds error")
	}
	if _, err := _decryptNcaHeader(nil, make([]byte, 0x10), 0x11, 0x10, 0); err == nil {
		t.Fatal("expected unaligned decryption bounds error")
	}
	if got := getFsEntry(&ncaHeader{headerBytes: make([]byte, 0x250)}, 1); got.Size != 0 {
		t.Fatal("expected invalid FS entry to be empty")
	}
	if _, err := getFsHeader(&ncaHeader{headerBytes: make([]byte, 0x3ff)}, 0); err == nil {
		t.Fatal("expected truncated FS header error")
	}
	if _, err := (&fsHeader{fsHeaderBytes: make([]byte, 0x100), hashType: 1}).getHashInfo(); err == nil {
		t.Fatal("expected unsupported hash error")
	}
}

func TestNcaHeaderFieldsAndRights(t *testing.T) {
	for _, test := range []struct {
		name string
		a    byte
		b    byte
		want byte
	}{
		{name: "first", a: 4, b: 2, want: 4},
		{name: "second", a: 2, b: 4, want: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := max(test.a, test.b); got != test.want {
				t.Fatalf("max(%d, %d) = %d", test.a, test.b, got)
			}
		})
	}
	if (&ncaHeader{rightsId: make([]byte, 0x10)}).HasRightsId() {
		t.Fatal("zero rights ID should be absent")
	}
	if !(&ncaHeader{rightsId: append([]byte{1}, make([]byte, 0xf)...)}).HasRightsId() {
		t.Fatal("non-zero rights ID should be present")
	}
	if (&ncaHeader{rightsId: []byte{}}).HasRightsId() || (*ncaHeader)(nil).HasRightsId() {
		t.Fatal("short rights ID should be absent")
	}

	plain := make([]byte, 0x400)
	copy(plain[0x200:], []byte("NCA2"))
	copy(plain[0x230:], bytes.Repeat([]byte{0x11}, 0x10))
	encrypted := encryptNcaHeader(plain, testHeaderKey)
	key, err := hex.DecodeString(testHeaderKey)
	if err != nil {
		t.Fatal(err)
	}
	c, err := newTestXTSCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := _decryptNcaHeader(c, encrypted, 0x400, 0x200, 0)
	if err != nil || !bytes.Equal(decoded[:0x400], plain) {
		t.Fatalf("synthetic header round trip failed: %v", err)
	}
	parsed, err := DecryptNcaHeader(testHeaderKey, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.HasRightsId() || string(parsed.rightsId) != string(bytes.Repeat([]byte{0x11}, 0x10)) {
		t.Fatalf("unexpected rights ID: %x", parsed.rightsId)
	}
}

func TestDecryptNcaHeaderSuccessAndBranches(t *testing.T) {
	plain := make([]byte, 0xc00)
	copy(plain[0x200:], []byte("NCA2"))
	binary.LittleEndian.PutUint64(plain[0x210:], 0x0102030405060708)
	plain[0x206] = 3
	plain[0x220] = 1
	parsed, err := DecryptNcaHeader(testHeaderKey, encryptNcaHeader(plain, testHeaderKey))
	if err != nil {
		t.Fatal(err)
	}
	if string(parsed.titleId) != "102030405060708" || parsed.getKeyRevision() != 2 {
		t.Fatalf("unexpected parsed NCA header: %#v", parsed)
	}

	nca3 := make([]byte, 0x400)
	copy(nca3[0x200:], []byte("NCA3"))
	if _, err := DecryptNcaHeader(testHeaderKey, encryptNcaHeader(nca3, testHeaderKey)); err == nil {
		t.Fatal("expected truncated NCA3 error")
	}
	invalid := make([]byte, 0x400)
	if _, err := DecryptNcaHeader(testHeaderKey, encryptNcaHeader(invalid, testHeaderKey)); err == nil {
		t.Fatal("expected invalid NCA magic error")
	}

	key, err := hex.DecodeString(testHeaderKey)
	c, err := newTestXTSCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	plain = bytes.Repeat([]byte{0x3c}, 0x200)
	encrypted := encryptNcaHeader(plain, testHeaderKey)
	decoded, err := _decryptNcaHeader(c, encrypted, len(plain), 0x200, 0)
	if err != nil || !bytes.Equal(decoded[:len(plain)], plain) {
		t.Fatalf("header sector decryption failed: %v", err)
	}
}

func TestFsHeaderAndEntry(t *testing.T) {
	headerBytes := make([]byte, 0x600)
	binary.LittleEndian.PutUint32(headerBytes[0x240:], 2)
	binary.LittleEndian.PutUint32(headerBytes[0x244:], 5)
	if got := getFsEntry(&ncaHeader{headerBytes: headerBytes}, 0); got.StartOffset != 0x400 || got.EndOffset != 0xa00 || got.Size != 0x600 {
		t.Fatalf("unexpected FS entry: %#v", got)
	}
	fs := headerBytes[0x400:0x600]
	fs[2] = 0
	fs[3] = 2
	binary.LittleEndian.PutUint64(fs[0x40:], 0x1234)
	binary.LittleEndian.PutUint64(fs[0x48:], 0x5678)
	sum := sha256Sum(fs)
	copy(headerBytes[0x280:], sum[:])
	parsed, err := getFsHeader(&ncaHeader{headerBytes: headerBytes}, 0)
	if err != nil {
		t.Fatal(err)
	}
	info, err := parsed.getHashInfo()
	if err != nil || info.pfs0HeaderOffset != 0x1234 || info.pfs0size != 0x5678 {
		t.Fatalf("unexpected hash info: %#v %v", info, err)
	}
	fs[3] = 9
	if _, err := getFsHeader(&ncaHeader{headerBytes: headerBytes}, 0); err == nil {
		t.Fatal("expected FS header hash mismatch")
	}
}

func TestDecryptAesCtrErrorPaths(t *testing.T) {
	if _, err := decryptAesCtr(&ncaHeader{cryptoType: 1}, &fsHeader{}, 0, 16, make([]byte, 16)); err == nil {
		t.Fatal("expected unsupported crypto type error")
	}
	if _, err := decryptAesCtr(&ncaHeader{cryptoType: 0}, &fsHeader{}, 0, 16, make([]byte, 16)); err == nil {
		t.Fatal("expected missing key error")
	}
	if _, _, err := openMetaNcaDataSection(bytes.NewReader(make([]byte, 0x10)), 0); err == nil {
		t.Fatal("expected short NCA error")
	}
}

func TestDecryptAesCtrSuccessAndTruncation(t *testing.T) {
	initTestKeys(t)
	plain := bytes.Repeat([]byte("ctr"), 32)
	nca := &ncaHeader{cryptoType: 0, keyGeneration1: 1, encryptedKeys: make([]byte, 0x40)}
	areaKey, _ := hex.DecodeString(testAreaKey)
	contentKey, _ := hex.DecodeString(testNcaKey)
	block, err := aes.NewCipher(areaKey)
	if err != nil {
		t.Fatal(err)
	}
	block.Encrypt(nca.encryptedKeys[0x20:0x30], contentKey)
	fs := &fsHeader{generation: 9}
	if got := nca.getKeyRevision(); got != 0 {
		t.Fatalf("unexpected key revision: %d", got)
	}
	encrypted := cryptCtr(t, contentKey, fs.generation, 0x30, plain)
	decoded, err := decryptAesCtr(nca, fs, 0x30, uint32(len(plain)), encrypted)
	if err != nil || !bytes.Equal(decoded, plain) {
		t.Fatalf("CTR decryption failed: %v", err)
	}
	if _, err := decryptAesCtr(nca, fs, 0, uint32(len(encrypted)+1), encrypted); err == nil {
		t.Fatal("expected truncated encoded section error")
	}
	shortKeys := *nca
	shortKeys.encryptedKeys = make([]byte, 0x2f)
	if _, err := decryptAesCtr(&shortKeys, fs, 0, 16, make([]byte, 16)); err == nil {
		t.Fatal("expected truncated encrypted keys error")
	}
	missingKeyDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(missingKeyDir, "prod.keys"), []byte("header_key = "+testHeaderKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := settings.InitSwitchKeys(missingKeyDir); err != nil {
		t.Fatal(err)
	}
	if _, err := decryptAesCtr(&ncaHeader{cryptoType: 0, encryptedKeys: make([]byte, 0x40)}, fs, 0, 16, make([]byte, 16)); err == nil {
		t.Fatal("expected missing application key error")
	}
}

func initTestKeys(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	keys := []byte("header_key = " + testHeaderKey + "\nkey_area_key_application_0 = " + testAreaKey + "\n")
	if err := os.WriteFile(filepath.Join(dir, "prod.keys"), keys, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := settings.InitSwitchKeys(dir); err != nil {
		t.Fatal(err)
	}
}

func makeSyntheticNCA(t *testing.T, section []byte, fsType byte) []byte {
	t.Helper()
	if len(section)%0x200 != 0 {
		t.Fatalf("synthetic NCA section is not sector aligned: %d", len(section))
	}
	initTestKeys(t)
	contentKey, _ := hex.DecodeString(testNcaKey)
	areaKey, _ := hex.DecodeString(testAreaKey)
	areaCipher, _ := aes.NewCipher(areaKey)
	header := make([]byte, 0xc00)
	copy(header[0x200:], []byte("NCA3"))
	binary.LittleEndian.PutUint32(header[0x240:], 6)
	binary.LittleEndian.PutUint32(header[0x244:], uint32(6+len(section)/0x200))
	header[0x402] = fsType
	header[0x403] = 2
	header[0x404] = 3
	binary.LittleEndian.PutUint32(header[0x540:], 1)
	binary.LittleEndian.PutUint64(header[0x440:], 0)
	binary.LittleEndian.PutUint64(header[0x448:], uint64(len(section)))
	fsHash := sha256.Sum256(header[0x400:0x600])
	copy(header[0x280:], fsHash[:])
	areaCipher.Encrypt(header[0x320:0x330], contentKey)
	encryptedHeader := encryptNcaHeader(header, testHeaderKey)
	encodedSection := cryptCtr(t, contentKey, 1, 0xc00, section)
	return append(encryptedHeader, encodedSection...)
}

func cryptCtr(t *testing.T, key []byte, generation uint32, offset uint32, data []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	counter := make([]byte, 0x10)
	binary.BigEndian.PutUint64(counter, uint64(generation))
	binary.BigEndian.PutUint64(counter[8:], uint64(offset/0x10))
	result := make([]byte, len(data))
	cipher.NewCTR(block, counter).XORKeyStream(result, data)
	return result
}

func encryptNcaHeader(plain []byte, keyString string) []byte {
	key, _ := hex.DecodeString(keyString)
	result := make([]byte, len(plain))
	for sector := 0; sector < len(plain)/0x200; sector++ {
		blockStart := sector * 0x200
		tweak := getNintendoTweak(sector)
		second, _ := aes.NewCipher(key[len(key)/2:])
		encryptedTweak := make([]byte, aes.BlockSize)
		second.Encrypt(encryptedTweak, tweak[:])
		for blockOffset := 0; blockOffset < 0x200; blockOffset += aes.BlockSize {
			block := encryptXTSBlock(key, plain[blockStart+blockOffset:blockStart+blockOffset+aes.BlockSize], encryptedTweak)
			copy(result[blockStart+blockOffset:blockStart+blockOffset+aes.BlockSize], block)
			testMul2Bytes(encryptedTweak)
		}
	}
	return result
}

func encryptXTSBlock(key, plain, tweak []byte) []byte {
	first, _ := aes.NewCipher(key[:len(key)/2])
	input := make([]byte, aes.BlockSize)
	for i := range input {
		input[i] = plain[i] ^ tweak[i]
	}
	first.Encrypt(input, input)
	for i := range input {
		input[i] ^= tweak[i]
	}
	return input
}

func newTestXTSCipher(key []byte) (*_crypto.Cipher, error) {
	return _crypto.NewCipher(aes.NewCipher, key)
}

func testMul2(tweak *[16]byte) {
	testMul2Bytes(tweak[:])
}

func testMul2Bytes(tweak []byte) {
	var carry byte
	for i := range tweak {
		next := tweak[i] >> 7
		tweak[i] = tweak[i]<<1 | carry
		carry = next
	}
	if carry != 0 {
		tweak[0] ^= 0x87
	}
}

func sha256Sum(data []byte) [32]byte {
	return sha256.Sum256(data)
}
