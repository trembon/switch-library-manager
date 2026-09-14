package _crypto

import "crypto/aes"

func DecryptAes128Ecb(data, key []byte) []byte {
	if len(key) != 16 || len(data)%aes.BlockSize != 0 {
		return nil
	}
	cipher, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	decrypted := make([]byte, len(data))
	size := 16

	for bs, be := 0, size; bs < len(data); bs, be = bs+size, be+size {
		cipher.Decrypt(decrypted[bs:be], data[bs:be])
	}

	return decrypted
}
