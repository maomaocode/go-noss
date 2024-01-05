package cmd

import (
	"crypto/rand"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip13"
	"strconv"
	"time"
)

func Generate(event nostr.Event, targetDifficulty int) (nostr.Event, error) {
	tag := nostr.Tag{"nonce", "", strconv.Itoa(targetDifficulty)}
	event.Tags = append(event.Tags, tag)
	start := time.Now()
	for {
		nonce, _ := generateRandomString(10)
		tag[1] = nonce
		event.CreatedAt = nostr.Now()
		if nip13.Difficulty(event.GetID()) >= targetDifficulty {
			//fmt.Print("calc cost: ", time.Since(start))
			return event, nil
		}

		costTime := time.Since(start)
		if costTime >= time.Duration(1*float32(time.Second)) {
			//fmt.Println(costTime)
			return event, ErrGenerateTimeout
		}
	}
}

func generateRandomString(length int) (string, error) {
	charset := "abcdefghijklmnopqrstuvwxyz0123456789" // 字符集
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	for i := 0; i < length; i++ {
		b[i] = charset[int(b[i])%len(charset)]

	}

	return string(b), nil
}